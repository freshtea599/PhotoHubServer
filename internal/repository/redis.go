// backend/internal/repository/redis.go
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/freshtea599/PhotoHubServer.git/internal/domain"
)

// RedisRepo управляет кэшем метаданных и статусами задач трансформации.
type RedisRepo struct {
	client *redis.Client
}

// NewRedisRepo создаёт новый экземпляр RedisRepo.
func NewRedisRepo(client *redis.Client) *RedisRepo {
	return &RedisRepo{client: client}
}

// CachePhotoMetadata сохраняет метаданные фото в Redis (ключ: "photo:<id>").
func (r *RedisRepo) CachePhotoMetadata(ctx context.Context, photo *domain.Photo) error {
	data, err := json.Marshal(photo)
	if err != nil {
		return fmt.Errorf("marshal photo metadata: %w", err)
	}
	key := fmt.Sprintf("photo:%d", photo.ID)
	return r.client.Set(ctx, key, data, 10*time.Minute).Err()
}

// GetPhotoMetadata получает метаданные фото из кэша.
func (r *RedisRepo) GetPhotoMetadata(ctx context.Context, photoID int64) (*domain.Photo, error) {
	key := fmt.Sprintf("photo:%d", photoID)
	data, err := r.client.Get(ctx, key).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // кэш пуст
		}
		return nil, fmt.Errorf("redis get photo metadata: %w", err)
	}
	var photo domain.Photo
	if err := json.Unmarshal(data, &photo); err != nil {
		return nil, fmt.Errorf("unmarshal photo metadata: %w", err)
	}
	return &photo, nil
}

// InvalidatePhoto удаляет метаданные фото из кэша.
func (r *RedisRepo) InvalidatePhoto(ctx context.Context, photoID int64) error {
	key := fmt.Sprintf("photo:%d", photoID)
	return r.client.Del(ctx, key).Err()
}

// SetJobStatus сохраняет статус задачи трансформации.
func (r *RedisRepo) SetJobStatus(ctx context.Context, jobID string, status string) error {
	key := fmt.Sprintf("job:%s", jobID)
	return r.client.Set(ctx, key, status, 5*time.Minute).Err()
}

// GetJobStatus получает статус задачи.
func (r *RedisRepo) GetJobStatus(ctx context.Context, jobID string) (string, error) {
	key := fmt.Sprintf("job:%s", jobID)
	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", err
	}
	return val, nil
}

// CacheVariantKey сохраняет ключ варианта (чтобы знать, есть ли уже готовая копия).
func (r *RedisRepo) CacheVariantKey(ctx context.Context, photoID int64, sizeName, format string, key string) error {
	redisKey := fmt.Sprintf("variant:%d:%s:%s", photoID, sizeName, format)
	return r.client.Set(ctx, redisKey, key, 10*time.Minute).Err()
}

// GetVariantKey получает ключ варианта из кэша.
func (r *RedisRepo) GetVariantKey(ctx context.Context, photoID int64, sizeName, format string) (string, error) {
	redisKey := fmt.Sprintf("variant:%d:%s:%s", photoID, sizeName, format)
	val, err := r.client.Get(ctx, redisKey).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", err
	}
	return val, nil
}
