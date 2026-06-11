-- +goose Up
-- +goose StatementBegin
CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT DEFAULT '',
    is_admin BOOLEAN DEFAULT FALSE,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE items (
    id SERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    price NUMERIC(10,2) DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    owner_id INTEGER NOT NULL REFERENCES users(id)
);

CREATE TABLE sessions (
    id SERIAL PRIMARY KEY,
    key_value TEXT UNIQUE NOT NULL,
    key_type TEXT DEFAULT 'session',
    scopes TEXT DEFAULT '',
    created_at INTEGER DEFAULT EXTRACT(EPOCH FROM NOW())::INTEGER,
    expires_at INTEGER DEFAULT 0,
    is_active BOOLEAN DEFAULT TRUE,
    for_user INTEGER NOT NULL REFERENCES users(id)
);

CREATE TABLE magic_links (
    id SERIAL PRIMARY KEY,
    token TEXT UNIQUE NOT NULL,
    email TEXT NOT NULL,
    created_at INTEGER DEFAULT EXTRACT(EPOCH FROM NOW())::INTEGER,
    expires_at INTEGER NOT NULL,
    used_at INTEGER DEFAULT 0,
    for_user INTEGER NOT NULL REFERENCES users(id)
);

CREATE INDEX idx_magic_links_token ON magic_links(token);
CREATE INDEX idx_magic_links_email ON magic_links(email);

-- Seed data for testing
INSERT INTO users (id, email, is_admin, is_active) VALUES
    (1, 'admin@example.com', TRUE, TRUE),
    (2, 'user@example.com', FALSE, TRUE);

-- Reset sequence after explicit ID inserts
SELECT setval('users_id_seq', (SELECT MAX(id) FROM users));

INSERT INTO sessions (key_value, key_type, for_user) VALUES
    ('admin-token', 'session', 1),
    ('user-token', 'session', 2);

INSERT INTO items (name, description, price, owner_id) VALUES
    ('Widget', 'A useful widget', 9.99, 1),
    ('Gadget', 'An amazing gadget', 19.99, 1),
    ('Gizmo', 'A fantastic gizmo', 29.99, 2);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS magic_links;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS items;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
