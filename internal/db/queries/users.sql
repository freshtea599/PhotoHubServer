-- internal/db/queries/users.sql

-- name: CreateUser :one
INSERT INTO users (username, is_admin)
VALUES ($1, $2)
RETURNING id, username, is_admin, created_at;

-- name: GetUserByID :one
SELECT id, username, is_admin, created_at
FROM users
WHERE id = $1;

-- name: GetUserByUsername :one
SELECT id, username, is_admin, created_at
FROM users
WHERE username = $1;

-- name: ListUsers :many
SELECT id, username, is_admin, created_at
FROM users
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;
