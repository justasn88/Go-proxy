CREATE TABLE IF NOT EXISTS users (
                                     id SERIAL PRIMARY KEY,
                                     username TEXT UNIQUE NOT NULL,
                                     password TEXT NOT NULL,
                                     data_limit_bytes BIGINT DEFAULT 1073741824,
                                     max_connections INTEGER DEFAULT 10
);

CREATE TABLE IF NOT EXISTS traffic_logs (
                                            id BIGSERIAL PRIMARY KEY,
                                            username TEXT REFERENCES users(username),
    bytes_used BIGINT,
    timestamp TIMESTAMPTZ DEFAULT NOW()
    );

INSERT INTO users (username, password) VALUES ('user', 'pass') ON CONFLICT DO NOTHING;