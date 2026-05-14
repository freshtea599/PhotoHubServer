// backend/internal/repository/minio.go
package repository

import (
	"context"
	"fmt"
	"io"

	"github.com/minio/minio-go/v7"
)

// MinioRepo инкапсулирует взаимодействие с MinIO.
type MinioRepo struct {
	client          *minio.Client
	bucketOriginals string
	bucketVariants  string
}

// NewMinioRepo создаёт новый экземпляр MinioRepo.
func NewMinioRepo(client *minio.Client, bucketOriginals, bucketVariants string) *MinioRepo {
	return &MinioRepo{
		client:          client,
		bucketOriginals: bucketOriginals,
		bucketVariants:  bucketVariants,
	}
}

// PutOriginal загружает оригинал изображения в бакет originals.
func (r *MinioRepo) PutOriginal(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	_, err := r.client.PutObject(ctx, r.bucketOriginals, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("failed to upload original %s: %w", key, err)
	}
	return nil
}

// GetOriginal возвращает io.ReadCloser для чтения оригинала из бакета originals.
func (r *MinioRepo) GetOriginal(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := r.client.GetObject(ctx, r.bucketOriginals, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get original %s: %w", key, err)
	}
	return obj, nil
}

// PutVariant загружает обработанный вариант изображения в бакет variants.
func (r *MinioRepo) PutVariant(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	_, err := r.client.PutObject(ctx, r.bucketVariants, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return fmt.Errorf("failed to upload variant %s: %w", key, err)
	}
	return nil
}

// GetVariant возвращает io.ReadCloser для чтения варианта из бакета variants.
func (r *MinioRepo) GetVariant(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := r.client.GetObject(ctx, r.bucketVariants, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get variant %s: %w", key, err)
	}
	return obj, nil
}

func (r *MinioRepo) DeleteOriginal(ctx context.Context, key string) error {
	return r.client.RemoveObject(ctx, r.bucketOriginals, key, minio.RemoveObjectOptions{})
}

func (r *MinioRepo) DeleteVariants(ctx context.Context, photoID int64) error {
	objectsCh := make(chan minio.ObjectInfo)
	go func() {
		defer close(objectsCh)
		for obj := range r.client.ListObjects(ctx, r.bucketVariants, minio.ListObjectsOptions{
			Prefix:    fmt.Sprintf("%d/", photoID),
			Recursive: true,
		}) {
			objectsCh <- obj
		}
	}()
	for err := range r.client.RemoveObjects(ctx, r.bucketVariants, objectsCh, minio.RemoveObjectsOptions{}) {
		if err.Err != nil {
			return err.Err
		}
	}
	return nil
}
