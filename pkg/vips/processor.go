package vips

import (
	"fmt"

	"github.com/h2non/bimg"
)

type Processor struct{}

func NewProcessor() (*Processor, error) {
	return &Processor{}, nil
}

// Transform выполняет ресайз и конвертацию в WebP/AVIF/JPEG.
func (p *Processor) Transform(data []byte, width int, format string, quality int) ([]byte, error) {
	img := bimg.NewImage(data)
	if img == nil {
		return nil, fmt.Errorf("bimg: nil image")
	}

	// Ресайз, если указана ширина
	if width > 0 {
		origSize, err := img.Size()
		if err != nil {
			return nil, fmt.Errorf("bimg size: %w", err)
		}
		height := int(float64(width) * float64(origSize.Height) / float64(origSize.Width))
		resized, err := img.Resize(width, height)
		if err != nil {
			return nil, fmt.Errorf("bimg resize: %w", err)
		}
		img = bimg.NewImage(resized)
	}

	// Выбор формата и сжатия
	switch format {
	case "webp":
		return img.Convert(bimg.WEBP)
	case "avif":
		opts := bimg.Options{Type: bimg.AVIF, Quality: quality}
		return img.Process(opts)
	default: // jpeg
		opts := bimg.Options{Type: bimg.JPEG, Quality: quality}
		return img.Process(opts)
	}
}
