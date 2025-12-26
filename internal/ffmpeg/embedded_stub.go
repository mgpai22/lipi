//go:build !ffmpeg_embedded

package ffmpeg

import "io"

func openEmbeddedAsset(name string) (io.ReadCloser, bool, error) {
	return nil, false, nil
}
