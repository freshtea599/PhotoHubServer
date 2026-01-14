-- migrations/000001_init.down.sql

DROP INDEX IF EXISTS idx_photos_is_public;
DROP INDEX IF EXISTS idx_photos_user_id;
DROP TABLE IF EXISTS photos;
DROP TABLE IF EXISTS users;
