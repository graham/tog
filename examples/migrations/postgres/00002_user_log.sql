-- +goose Up
-- +goose StatementBegin
CREATE TABLE user_logs (
    id SERIAL PRIMARY KEY,
    author_id INTEGER NOT NULL REFERENCES users(id),
    message TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_user_logs_author ON user_logs(author_id);
CREATE INDEX idx_user_logs_created ON user_logs(created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS user_logs;
-- +goose StatementEnd
