-- internal/db/queries/photos.sql

-- name: CreatePhoto :one
INSERT INTO photos (url, user_id, description, is_public)
VALUES ($1, $2, $3, $4)
RETURNING id, url, user_id, description, is_public, likes_count, created_at;

-- name: GetPhotoByID :one
SELECT id, url, user_id, description, is_public, likes_count, created_at
FROM photos
WHERE id = $1;

-- name: ListPhotos :many
SELECT id, url, user_id, description, is_public, likes_count, created_at
FROM photos
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListPhotosByUserID :many
SELECT id, url, user_id, description, is_public, likes_count, created_at
FROM photos
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: DeletePhoto :exec
DELETE FROM photos WHERE id = $1;

-- name: UpdatePhotoLikes :one
UPDATE photos
SET likes_count = likes_count + $1
WHERE id = $2
RETURNING id, url, user_id, description, is_public, likes_count, created_at;
