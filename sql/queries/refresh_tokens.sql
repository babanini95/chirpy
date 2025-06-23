-- name: GetRefreshToken :one
SELECT *
FROM refresh_tokens
WHERE user_id = $1;

-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (
        id,
        created_at,
        updated_at,
        user_id,
        expires_at,
        revoked_at
    )
VALUES (gen_random_uuid(), NOW(), NOW(), $1, $2, NULL)
RETURNING *;