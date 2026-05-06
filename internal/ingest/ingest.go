// SPDX-License-Identifier: MPL-2.0

/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package ingest

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/seborama/ragna-rock/internal/chunking"
	"github.com/seborama/ragna-rock/internal/ollama"
	"github.com/seborama/ragna-rock/internal/rag"
)

type Service struct {
	DB         *sql.DB
	Ollama     *ollama.Client
	EmbedModel string
}

type Stats struct {
	Documents int
	Chunks    int
}

func (s Service) IngestPath(ctx context.Context, root string) (Stats, error) {
	var stats Stats
	info, err := os.Stat(root)
	if err != nil {
		return stats, err
	}
	if !info.IsDir() {
		return s.ingestFile(ctx, root)
	}

	err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !supported(path) {
			return nil
		}
		st, err := s.ingestFile(ctx, path)
		if err != nil {
			return err
		}
		stats.Documents += st.Documents
		stats.Chunks += st.Chunks
		return nil
	})
	return stats, err
}

func (s Service) ingestFile(ctx context.Context, path string) (Stats, error) {
	var stats Stats
	if !supported(path) {
		return stats, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return stats, err
	}
	text := strings.TrimSpace(string(b))
	if text == "" {
		return stats, nil
	}

	docID := rag.StableID("doc", path)
	name := filepath.Base(path)
	topic := filepath.Base(filepath.Dir(path))
	chunks := chunking.Default().Split(text)
	if len(chunks) == 0 {
		return stats, nil
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return stats, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO documents (id, name, source_path, topic, updated_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (source_path) DO UPDATE
		SET name = EXCLUDED.name, topic = EXCLUDED.topic, updated_at = now()
	`, docID, name, path, topic)
	if err != nil {
		return stats, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM chunks WHERE document_id = $1`, docID); err != nil {
		return stats, err
	}

	for _, ch := range chunks {
		emb, err := s.Ollama.Embedding(ctx, s.EmbedModel, ch.Text)
		if err != nil {
			return stats, fmt.Errorf("embed %s chunk %d: %w", path, ch.Index, err)
		}
		chunkID := rag.StableID("chunk", fmt.Sprintf("%s:%d:%s", path, ch.Index, ch.Text))
		_, err = tx.ExecContext(ctx, `
			INSERT INTO chunks
			(id, document_id, document_name, source_path, topic, section, chunk_index, text, embedding)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9::vector)
		`, chunkID, docID, name, path, topic, ch.Section, ch.Index, ch.Text, rag.VectorLiteral(emb))
		if err != nil {
			return stats, fmt.Errorf("insert chunk %d from %s: %w", ch.Index, path, err)
		}
		stats.Chunks++
	}
	if err := tx.Commit(); err != nil {
		return stats, err
	}
	stats.Documents = 1
	return stats, nil
}

func supported(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".txt", ".md":
		return true
	default:
		return false
	}
}
