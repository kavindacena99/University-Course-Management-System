DROP TABLE IF EXISTS user_status_history;

ALTER TABLE users
    DROP CONSTRAINT IF EXISTS chk_users_version_positive,
    DROP COLUMN IF EXISTS version,
    DROP COLUMN IF EXISTS reviewed_at,
    DROP COLUMN IF EXISTS reviewed_by,
    DROP COLUMN IF EXISTS status_reason;
