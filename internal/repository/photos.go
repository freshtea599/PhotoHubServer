package repository

import (
	"errors"
	"sync"

	"github.com/freshtea599/PhotoHubServer.git/internal/models"
)

var ErrPhotoNotFound = errors.New("photo not found")

type PhotoRepository struct {
	mu     sync.RWMutex
	data   []models.Photo
	nextID int64
}

func NewPhotoRepository() *PhotoRepository {
	return &PhotoRepository{
		data:   make([]models.Photo, 0),
		nextID: 1,
	}
}

// List: пагинация.
func (r *PhotoRepository) List(limit, offset int) []models.Photo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if offset >= len(r.data) {
		return []models.Photo{}
	}

	end := offset + limit
	if end > len(r.data) {
		end = len(r.data)
	}

	return append([]models.Photo(nil), r.data[offset:end]...)
}

// Create: добавление.
func (r *PhotoRepository) Create(p models.Photo) models.Photo {
	r.mu.Lock()
	defer r.mu.Unlock()

	p.ID = r.nextID
	r.nextID++
	r.data = append(r.data, p)
	return p
}

// GetByID: поиск.
func (r *PhotoRepository) GetByID(id int64) (models.Photo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.data {
		if p.ID == id {
			return p, nil
		}
	}
	return models.Photo{}, ErrPhotoNotFound
}

// Delete: удаление.
func (r *PhotoRepository) Delete(id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for i, p := range r.data {
		if p.ID == id {
			r.data = append(r.data[:i], r.data[i+1:]...)
			return nil
		}
	}
	return ErrPhotoNotFound
}
