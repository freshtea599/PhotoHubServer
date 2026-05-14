// backend/internal/usecase/image_processor.go
package usecase

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/google/uuid"

	"github.com/freshtea599/PhotoHubServer.git/internal/domain"
	"github.com/freshtea599/PhotoHubServer.git/internal/repository"
	vipsproc "github.com/freshtea599/PhotoHubServer.git/pkg/vips"
)

// ImageProcessor управляет JIT-трансформацией изображений.
type ImageProcessor struct {
	pool      *WorkerPool
	redisRepo *repository.RedisRepo
	minioRepo *repository.MinioRepo
	photoRepo *repository.PostgresPhotoRepo
	vipsProc  *vipsproc.Processor
}

// NewImageProcessor создаёт новый процессор, внутри запускает Worker Pool.
func NewImageProcessor(
	numWorkers int,
	vipsProc *vipsproc.Processor,
	minioRepo *repository.MinioRepo,
	redisRepo *repository.RedisRepo,
	photoRepo *repository.PostgresPhotoRepo,
) (*ImageProcessor, error) {
	ip := &ImageProcessor{
		redisRepo: redisRepo,
		minioRepo: minioRepo,
		photoRepo: photoRepo,
		vipsProc:  vipsProc,
	}

	// Создаём пул, передавая замыкание-обработчик
	ip.pool = NewWorkerPool(numWorkers, ip.processJob)
	ip.pool.Start()

	return ip, nil
}

// Shutdown останавливает пул.
func (ip *ImageProcessor) Shutdown() {
	ip.pool.Shutdown()
}

// GetVariant возвращает байты варианта изображения для указанного photoID и параметров.
func (ip *ImageProcessor) GetVariant(
	ctx context.Context,
	photoID int64,
	sizeName string,
	width int,
	format string,
	quality int,
) ([]byte, string, error) {
	// 1. Проверяем кэш Redis (ключ варианта)
	cachedKey, err := ip.redisRepo.GetVariantKey(ctx, photoID, sizeName, format)
	if err != nil {
		log.Printf("redis get variant key error: %v", err)
	}
	if cachedKey != "" {
		// 2. Читаем готовый вариант из MinIO
		reader, err := ip.minioRepo.GetVariant(ctx, cachedKey)
		if err == nil {
			defer reader.Close()
			data, err := io.ReadAll(reader)
			if err == nil {
				contentType := mimeTypeForFormat(format)
				return data, contentType, nil
			}
		}
		log.Printf("cached variant not found in MinIO, will regenerate: %s", cachedKey)
	}

	// 3. Проверяем, что фото существует (без сохранения в переменную)
	if _, err := ip.photoRepo.GetByID(photoID); err != nil {
		return nil, "", fmt.Errorf("photo not found: %w", err)
	}

	// 4. Формируем Job с высоким приоритетом
	job := domain.Job{
		ID:        uuid.New(),
		PhotoID:   photoID,
		Width:     width,
		Format:    format,
		Quality:   quality,
		Priority:  domain.PriorityHigh,
		CreatedAt: time.Now().Unix(),
	}

	// 5. Отправляем задачу в пул и ждём результат
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	result, err := ip.pool.Submit(ctx, job)
	if err != nil {
		return nil, "", fmt.Errorf("job submission failed: %w", err)
	}
	if result.Err != nil {
		return nil, "", fmt.Errorf("processing error: %w", result.Err)
	}

	// 6. Сохраняем результат в MinIO
	variantKey := fmt.Sprintf("%d/%s/%s/%s", photoID, sizeName, format, job.ID.String())
	err = ip.minioRepo.PutVariant(ctx, variantKey, bytes.NewReader(result.Data), int64(len(result.Data)), mimeTypeForFormat(format))
	// После строки err = ip.minioRepo.PutVariant(...)
	if err == nil {
		// Сохраняем запись в БД
		variant := &domain.PhotoVariant{
			PhotoID:  photoID,
			SizeName: sizeName,
			Format:   format,
			FilePath: variantKey,
			Width:    width,
			Quality:  quality,
		}
		if dbErr := ip.photoRepo.CreateVariant(variant); dbErr != nil {
			log.Printf("DB variant save error: %v", dbErr)
		}
		// Кэшируем ключ в Redis
		if cacheErr := ip.redisRepo.CacheVariantKey(ctx, photoID, sizeName, format, variantKey); cacheErr != nil {
			log.Printf("Redis cache error: %v", cacheErr)
		}
	}
	if err != nil {
		log.Printf("failed to save variant to MinIO: %v", err)
	} else {
		if cacheErr := ip.redisRepo.CacheVariantKey(ctx, photoID, sizeName, format, variantKey); cacheErr != nil {
			log.Printf("failed to cache variant key in Redis: %v", cacheErr)
		}
	}

	return result.Data, mimeTypeForFormat(format), nil
}

// processJob – реальная обработка внутри воркера.
func (ip *ImageProcessor) processJob(job domain.Job) domain.JobResult {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	photo, err := ip.photoRepo.GetByID(job.PhotoID)
	if err != nil {
		return domain.JobResult{Job: job, Err: fmt.Errorf("photo not found: %w", err)}
	}

	reader, err := ip.minioRepo.GetOriginal(ctx, photo.FilePath)
	if err != nil {
		return domain.JobResult{Job: job, Err: fmt.Errorf("failed to read original: %w", err)}
	}
	defer reader.Close()
	originalBytes, err := io.ReadAll(reader)
	if err != nil {
		return domain.JobResult{Job: job, Err: fmt.Errorf("failed to read original data: %w", err)}
	}

	data, err := ip.vipsProc.Transform(originalBytes, job.Width, job.Format, job.Quality)
	if err != nil {
		return domain.JobResult{Job: job, Err: fmt.Errorf("vips transform error: %w", err)}
	}

	return domain.JobResult{
		Job:  job,
		Data: data,
	}
}

func mimeTypeForFormat(format string) string {
	switch format {
	case "webp":
		return "image/webp"
	case "avif":
		return "image/avif"
	default:
		return "image/jpeg"
	}
}
