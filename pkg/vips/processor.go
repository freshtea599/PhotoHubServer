package vips

import (
	"fmt"
	"log"

	vips "github.com/davidbyttow/govips/v2/vips"
)

type Processor struct{}

func NewProcessor() (*Processor, error) {
	// Инициализация libvips с настройками по умолчанию
	vips.Startup(nil)
	// Необязательное логирование (если не сработает — закомментируйте)
	vips.LoggingSettings(func(messageDomain string, messageLevel vips.LogLevel, message string) {
		log.Printf("[vips] %s: %s", messageDomain, message)
	}, vips.LogLevelWarning)
	return &Processor{}, nil
}

// Transform принимает байты изображения, желаемую ширину, формат и качество.
// Возвращает байты трансформированного изображения.
func (p *Processor) Transform(data []byte, width int, format string, quality int) ([]byte, error) {
	img, err := vips.NewImageFromBuffer(data)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	defer img.Close()

	// Ресайз по ширине, сохраняя пропорции
	if width > 0 && width < img.Width() {
		scale := float64(width) / float64(img.Width())
		if err := img.Resize(scale, vips.KernelLanczos3); err != nil {
			return nil, fmt.Errorf("resize: %w", err)
		}
	}

	// Экспорт в целевой формат
	switch format {
	case "webp":
		ep := vips.WebpExportParams{Quality: quality}
		result, _, err := img.ExportWebp(&ep)
		return result, err
	case "avif":
		ep := vips.AvifExportParams{Quality: quality}
		result, _, err := img.ExportAvif(&ep)
		return result, err
	default: // jpeg
		ep := vips.JpegExportParams{Quality: quality, StripMetadata: true}
		result, _, err := img.ExportJpeg(&ep)
		return result, err
	}
}

// Shutdown завершает работу libvips
func (p *Processor) Shutdown() {
	vips.Shutdown()
}
