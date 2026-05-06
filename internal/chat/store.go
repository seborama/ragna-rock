// SPDX-License-Identifier: MPL-2.0

/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package chat

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/seborama/ragna-rock/internal/rag"
)

type Store struct {
	DB *sql.DB
}

func (s Store) GetOrCreateSession(ctx context.Context, id string) (rag.ChatSession, error) {
	if strings.TrimSpace(id) == "" {
		id = rag.NewID("session")
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO chat_sessions (id, title)
		VALUES ($1, $2)
		ON CONFLICT (id) DO NOTHING
	`, id, id)
	if err != nil {
		return rag.ChatSession{}, err
	}
	return s.GetSession(ctx, id)
}

func (s Store) GetSession(ctx context.Context, id string) (rag.ChatSession, error) {
	var sess rag.ChatSession
	var topicsRaw []byte
	err := s.DB.QueryRowContext(ctx, `
		SELECT id, title, summary, active_topics, created_at, updated_at
		FROM chat_sessions
		WHERE id = $1
	`, id).Scan(&sess.ID, &sess.Title, &sess.Summary, &topicsRaw, &sess.CreatedAt, &sess.UpdatedAt)
	if err != nil {
		return rag.ChatSession{}, err
	}
	_ = json.Unmarshal(topicsRaw, &sess.ActiveTopics)
	return sess, nil
}

func (s Store) AddTurn(ctx context.Context, sessionID, role, content string) error {
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO chat_turns (id, session_id, role, content)
		VALUES ($1, $2, $3, $4)
	`, rag.NewID("turn"), sessionID, role, content)
	return err
}

func (s Store) RecentTurns(ctx context.Context, sessionID string, limit int) ([]rag.ChatTurn, error) {
	if limit <= 0 {
		limit = 6
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, session_id, role, content, created_at
		FROM (
		    SELECT id, session_id, role, content, created_at
		    FROM chat_turns
		    WHERE session_id = $1
		    ORDER BY created_at DESC
		    LIMIT $2
		) recent
		ORDER BY created_at ASC
	`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var turns []rag.ChatTurn
	for rows.Next() {
		var t rag.ChatTurn
		if err := rows.Scan(&t.ID, &t.SessionID, &t.Role, &t.Content, &t.CreatedAt); err != nil {
			return nil, err
		}
		turns = append(turns, t)
	}
	return turns, rows.Err()
}

func (s Store) UpdateSessionMemory(ctx context.Context, sessionID, summary string, topics []string) error {
	b, err := json.Marshal(topics)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx, `
		UPDATE chat_sessions
		SET summary = $2,
		    active_topics = $3::jsonb,
		    updated_at = now()
		WHERE id = $1
	`, sessionID, strings.TrimSpace(summary), string(b))
	return err
}

func (s Store) MarkSeenChunks(ctx context.Context, sessionID string, chunks []rag.Chunk) error {
	for _, ch := range chunks {
		_, err := s.DB.ExecContext(ctx, `
			INSERT INTO chat_seen_chunks (session_id, chunk_id, use_count, last_used_at)
			VALUES ($1, $2, 1, now())
			ON CONFLICT (session_id, chunk_id) DO UPDATE
			SET use_count = chat_seen_chunks.use_count + 1,
			    last_used_at = now()
		`, sessionID, ch.ID)
		if err != nil {
			return fmt.Errorf("mark chunk %s seen: %w", ch.ID, err)
		}
	}
	return nil
}
