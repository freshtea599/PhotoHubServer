package repository

import (
	"database/sql"
	"errors"

	"github.com/freshtea599/PhotoHubServer.git/internal/models"
	"golang.org/x/crypto/bcrypt"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

// хеширует пароль
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// проверяет пароль
func checkPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// создаёт нового пользователя
func (r *UserRepository) Create(email, password, username string) (*models.User, error) {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return nil, err
	}

	var user models.User
	err = r.db.QueryRow(
		`INSERT INTO users (email, password_hash, username, created_at, updated_at) 
		 VALUES ($1, $2, $3, NOW(), NOW()) 
		 RETURNING id, email, username, created_at, updated_at`,
		email, hashedPassword, username,
	).Scan(&user.ID, &user.Email, &user.Username, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return nil, err
	}

	return &user, nil
}

// получает пользователя по email
func (r *UserRepository) GetByEmail(email string) (*models.User, string, error) {
	var user models.User
	var passwordHash string

	err := r.db.QueryRow(
		`SELECT id, email, username, is_admin, password_hash, created_at, updated_at
		 FROM users WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Email, &user.Username, &user.IsAdmin, &passwordHash, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, "", errors.New("user not found")
		}
		return nil, "", err
	}

	return &user, passwordHash, nil
}

// получает пользователя по ID
func (r *UserRepository) GetByID(id int64) (*models.User, error) {
	var user models.User
	err := r.db.QueryRow(
		`SELECT id, email, username, is_admin, created_at, updated_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Email, &user.Username, &user.IsAdmin, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

// проверяет пароль пользователя
func (r *UserRepository) CheckPassword(email, password string) (*models.User, error) {
	user, hash, err := r.GetByEmail(email)
	if err != nil {
		return nil, err
	}

	if !checkPassword(hash, password) {
		return nil, errors.New("invalid password")
	}

	return user, nil
}
