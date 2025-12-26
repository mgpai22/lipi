package ffmpeg

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

const (
	ffmpegReleaseVersion = "6.1"
	ffmpegReleaseBaseURL = "https://github.com/ffbinaries/ffbinaries-prebuilt/releases/download"
)

type BinaryPaths struct {
	FFmpeg  string
	FFprobe string
}

var (
	ensureOnce sync.Once
	ensureErr  error
	ensurePath BinaryPaths
)

func Ensure() (BinaryPaths, error) {
	ensureOnce.Do(func() {
		ensurePath, ensureErr = ensure()
	})
	return ensurePath, ensureErr
}

func FFmpegPath() (string, error) {
	paths, err := Ensure()
	if err != nil {
		return "", err
	}
	return paths.FFmpeg, nil
}

func FFprobePath() (string, error) {
	paths, err := Ensure()
	if err != nil {
		return "", err
	}
	return paths.FFprobe, nil
}

func ensure() (BinaryPaths, error) {
	paths := BinaryPaths{}
	ffmpegPath := os.Getenv("LIPI_FFMPEG_PATH")
	ffprobePath := os.Getenv("LIPI_FFPROBE_PATH")
	if ffmpegPath != "" && ffprobePath != "" {
		return BinaryPaths{FFmpeg: ffmpegPath, FFprobe: ffprobePath}, nil
	}

	if ffmpegPath == "" {
		if found, err := exec.LookPath("ffmpeg"); err == nil {
			ffmpegPath = found
		}
	}
	if ffprobePath == "" {
		if found, err := exec.LookPath("ffprobe"); err == nil {
			ffprobePath = found
		}
	}

	if ffmpegPath != "" && ffprobePath != "" {
		paths.FFmpeg = ffmpegPath
		paths.FFprobe = ffprobePath
		return paths, nil
	}

	assetName, err := assetForPlatform(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return BinaryPaths{}, err
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		cacheDir = os.TempDir()
	}
	installDir := filepath.Join(
		cacheDir,
		"lipi",
		"ffmpeg",
		ffmpegReleaseVersion,
		runtime.GOOS,
		runtime.GOARCH,
	)
	exeSuffix := executableSuffix()
	ffmpegPath = filepath.Join(installDir, "ffmpeg"+exeSuffix)
	ffprobePath = filepath.Join(installDir, "ffprobe"+exeSuffix)

	if binariesExist(ffmpegPath, ffprobePath) {
		return BinaryPaths{FFmpeg: ffmpegPath, FFprobe: ffprobePath}, nil
	}

	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return BinaryPaths{}, fmt.Errorf("create ffmpeg cache dir: %w", err)
	}

	embeddedUsed, err := extractEmbedded(assetName, installDir)
	if err != nil {
		return BinaryPaths{}, err
	}
	if embeddedUsed {
		if !binariesExist(ffmpegPath, ffprobePath) {
			return BinaryPaths{}, errors.New("embedded ffmpeg binaries missing after extraction")
		}
		if runtime.GOOS != "windows" {
			if err := os.Chmod(ffmpegPath, 0o755); err != nil {
				return BinaryPaths{}, fmt.Errorf("chmod ffmpeg: %w", err)
			}
			if err := os.Chmod(ffprobePath, 0o755); err != nil {
				return BinaryPaths{}, fmt.Errorf("chmod ffprobe: %w", err)
			}
		}
		return BinaryPaths{FFmpeg: ffmpegPath, FFprobe: ffprobePath}, nil
	}

	if err := downloadAndExtract(assetName, installDir); err != nil {
		return BinaryPaths{}, err
	}

	if !binariesExist(ffmpegPath, ffprobePath) {
		return BinaryPaths{}, errors.New("ffmpeg binaries not found after extraction")
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(ffmpegPath, 0o755); err != nil {
			return BinaryPaths{}, fmt.Errorf("chmod ffmpeg: %w", err)
		}
		if err := os.Chmod(ffprobePath, 0o755); err != nil {
			return BinaryPaths{}, fmt.Errorf("chmod ffprobe: %w", err)
		}
	}

	return BinaryPaths{FFmpeg: ffmpegPath, FFprobe: ffprobePath}, nil
}

func assetForPlatform(goos, goarch string) (string, error) {
	switch {
	case goos == "linux" && goarch == "amd64":
		return "ffmpeg-" + ffmpegReleaseVersion + "-linux-64.zip", nil
	case goos == "linux" && goarch == "arm64":
		return "ffmpeg-" + ffmpegReleaseVersion + "-linux-arm-64.zip", nil
	case goos == "darwin" && goarch == "amd64":
		return "ffmpeg-" + ffmpegReleaseVersion + "-macos-64.zip", nil
	case goos == "windows" && goarch == "amd64":
		return "ffmpeg-" + ffmpegReleaseVersion + "-win-64.zip", nil
	default:
		return "", fmt.Errorf("unsupported platform for bundled ffmpeg: %s/%s", goos, goarch)
	}
}

func downloadAndExtract(assetName, installDir string) error {
	url := fmt.Sprintf("%s/v%s/%s", ffmpegReleaseBaseURL, ffmpegReleaseVersion, assetName)
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download ffmpeg bundle: %w", err)
	}
	if resp == nil {
		return errors.New("download ffmpeg bundle: nil response")
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download ffmpeg bundle: unexpected status %s", resp.Status)
	}

	return extractArchiveFromReader(assetName, resp.Body, installDir)
}

func extractEmbedded(assetName, installDir string) (bool, error) {
	reader, ok, err := openEmbeddedAsset(assetName)
	if err != nil || !ok {
		return ok, err
	}
	defer func() { _ = reader.Close() }()

	if err := extractArchiveFromReader(assetName, reader, installDir); err != nil {
		return true, err
	}
	return true, nil
}

func extractArchiveFromReader(assetName string, reader io.Reader, installDir string) error {
	tmpFile, err := os.CreateTemp("", "lipi-ffmpeg-*.zip")
	if err != nil {
		return fmt.Errorf("create temp archive: %w", err)
	}
	archivePath := tmpFile.Name()
	if _, err := io.Copy(tmpFile, reader); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(archivePath)
		return fmt.Errorf("write archive: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(archivePath)
		return fmt.Errorf("close archive: %w", err)
	}
	defer func() { _ = os.Remove(archivePath) }()

	if err := extractArchive(archivePath, installDir); err != nil {
		return fmt.Errorf("extract %s: %w", assetName, err)
	}
	return nil
}

func extractArchive(archivePath, installDir string) error {
	zipReader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("open ffmpeg archive: %w", err)
	}
	defer func() { _ = zipReader.Close() }()

	ffmpegFound := false
	ffprobeFound := false
	for _, file := range zipReader.File {
		name := filepath.Base(file.Name)
		if isFFmpegBinary(name) {
			dest := filepath.Join(installDir, "ffmpeg"+executableSuffix())
			if err := extractZipFile(file, dest); err != nil {
				return err
			}
			ffmpegFound = true
			continue
		}
		if isFFprobeBinary(name) {
			dest := filepath.Join(installDir, "ffprobe"+executableSuffix())
			if err := extractZipFile(file, dest); err != nil {
				return err
			}
			ffprobeFound = true
		}
	}

	if !ffmpegFound || !ffprobeFound {
		return fmt.Errorf("ffmpeg archive missing required binaries")
	}

	return nil
}

func extractZipFile(file *zip.File, dest string) error {
	reader, err := file.Open()
	if err != nil {
		return fmt.Errorf("open ffmpeg archive entry: %w", err)
	}
	defer func() { _ = reader.Close() }()

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("create ffmpeg output dir: %w", err)
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create ffmpeg binary: %w", err)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, reader); err != nil {
		return fmt.Errorf("write ffmpeg binary: %w", err)
	}
	return nil
}

func binariesExist(ffmpegPath, ffprobePath string) bool {
	return fileExists(ffmpegPath) && fileExists(ffprobePath)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir() && info.Size() > 0
}

func isFFmpegBinary(name string) bool {
	name = strings.ToLower(name)
	return name == "ffmpeg" || name == "ffmpeg.exe"
}

func isFFprobeBinary(name string) bool {
	name = strings.ToLower(name)
	return name == "ffprobe" || name == "ffprobe.exe"
}

func executableSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}
