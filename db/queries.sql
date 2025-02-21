-- name: CreateUser :one
INSERT INTO users (google_id, email, name, picture)
VALUES (?, ?, ?, ?)
RETURNING id, google_id, email, name, picture, created_at;

-- name: GetUserByGoogleID :one
SELECT id, google_id, email, name, picture, created_at
FROM users
WHERE google_id = ?
LIMIT 1;
