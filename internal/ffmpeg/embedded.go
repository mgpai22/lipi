//go:build ffmpeg_embedded

package ffmpeg

import (
	"embed"
	"errors"
	"io"
	"io/fs"
)

//go:embed assets/*
var embeddedAssets embed.FS

func openEmbeddedAsset(name string) (io.ReadCloser, bool, error) {
	file, err := embeddedAssets.Open("assets/" + name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return file, true, nil
}
