package repository

import (
	"errors"
	"sync"

	"github.com/freshtea599/PhotoHubServer.git/internal/models"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepository struct {
	mu     sync.RWMutex
	data   []models.User
	nextID int64
}

func NewUserRepository() *UserRepository {
	return &UserRepository{
		data:   make([]models.User, 0),
		nextID: 1,
	}
}

func (r *UserRepository) Create(u models.User) models.User {
	r.mu.Lock()
	defer r.mu.Unlock()

	u.ID = r.nextID
	r.nextID++
	r.data = append(r.data, u)
	return u
}

func (r *UserRepository) GetByID(id int64) (models.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, u := range r.data {
		if u.ID == id {
			return u, nil
		}
	}
	return models.User{}, ErrUserNotFound
}
