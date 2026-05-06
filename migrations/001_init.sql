-- SPDX-License-Identifier: MPL-2.0
--
-- This Source Code Form is subject to the terms of the Mozilla Public
-- License, v. 2.0. If a copy of the MPL was not distributed with this
-- file, You can obtain one at https://mozilla.org/MPL/2.0/.

CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS documents (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    source_path TEXT NOT NULL UNIQUE,
    topic       TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Default dimension is 1024 for bge-m3. If you use another embedding model,
-- change vector(1024) before running migrate.
CREATE TABLE IF NOT EXISTS chunks (
    id             TEXT PRIMARY KEY,
    document_id    TEXT NOT NULL REFERENCES documents(id) ON DELETE CASCADE,
    document_name  TEXT NOT NULL,
    source_path    TEXT NOT NULL,
    topic          TEXT NOT NULL DEFAULT '',
    section        TEXT NOT NULL DEFAULT '',
    chunk_index    INT NOT NULL,
    text           TEXT NOT NULL,
    embedding      vector(1024) NOT NULL,
    full_text      TSVECTOR GENERATED ALWAYS AS (to_tsvector('english', coalesce(text, ''))) STORED,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (document_id, chunk_index)
);

CREATE INDEX IF NOT EXISTS chunks_embedding_hnsw_idx
    ON chunks USING hnsw (embedding vector_cosine_ops);

CREATE INDEX IF NOT EXISTS chunks_full_text_gin_idx
    ON chunks USING gin (full_text);

CREATE INDEX IF NOT EXISTS chunks_document_idx
    ON chunks (document_id, chunk_index);

CREATE TABLE IF NOT EXISTS chat_sessions (
    id            TEXT PRIMARY KEY,
    title         TEXT NOT NULL DEFAULT '',
    summary       TEXT NOT NULL DEFAULT '',
    active_topics JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS chat_turns (
    id         TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('user', 'assistant')),
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS chat_turns_session_created_idx
    ON chat_turns (session_id, created_at DESC);

CREATE TABLE IF NOT EXISTS chat_seen_chunks (
    session_id   TEXT NOT NULL REFERENCES chat_sessions(id) ON DELETE CASCADE,
    chunk_id      TEXT NOT NULL REFERENCES chunks(id) ON DELETE CASCADE,
    use_count     INT NOT NULL DEFAULT 1,
    first_used_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (session_id, chunk_id)
);

CREATE INDEX IF NOT EXISTS chat_seen_chunks_recent_idx
    ON chat_seen_chunks (session_id, last_used_at DESC);
