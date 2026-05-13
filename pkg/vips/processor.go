// backend/pkg/vips/processor.go
package vips

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg" // для поддержки JPEG
	_ "image/png"  // для поддержки PNG

	"github.com/disintegration/imaging"
)

type Processor struct{}

func NewProcessor() (*Processor, error) {
	return &Processor{}, nil
}

func (p *Processor) Transform(data []byte, width int, format string, quality int) ([]byte, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}

	if width > 0 {
		img = imaging.Resize(img, width, 0, imaging.Lanczos)
	}

	var buf bytes.Buffer
	switch format {
	case "webp":
		// imaging не поддерживает WebP, поэтому отдаём JPEG с тем же качеством
		fallthrough
	default:
		err = imaging.Encode(&buf, img, imaging.JPEG, imaging.JPEGQuality(quality))
	}
	if err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	return buf.Bytes(), nil
}
