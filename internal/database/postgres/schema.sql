CREATE TABLE IF NOT EXISTS short_urls (
    id           BIGINT PRIMARY KEY,
    long_url     TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ,
    click_count  BIGINT NOT NULL DEFAULT 0
);