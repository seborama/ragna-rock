// SPDX-License-Identifier: MPL-2.0

/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package rag

import (
	"context"
	"database/sql"
	"fmt"
)

type Searcher struct {
	DB *sql.DB
}

type SearchOptions struct {
	Candidates int
	TopK       int
	RRFK       int
}

func (s Searcher) HybridSearch(ctx context.Context, query string, embedding []float64, opt SearchOptions) ([]Chunk, error) {
	if opt.Candidates <= 0 {
		opt.Candidates = 24
	}
	if opt.TopK <= 0 {
		opt.TopK = 8
	}
	if opt.RRFK <= 0 {
		opt.RRFK = 60
	}
	rows, err := s.DB.QueryContext(ctx, hybridSQL, VectorLiteral(embedding), opt.Candidates, query, opt.RRFK, opt.TopK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []Chunk
	for rows.Next() {
		var ch Chunk
		var vectorRank, lexicalRank sql.NullInt64
		var vectorSimilarity, lexicalScore sql.NullFloat64
		if err := rows.Scan(
			&ch.ID,
			&ch.DocumentID,
			&ch.DocumentName,
			&ch.SourcePath,
			&ch.Topic,
			&ch.Section,
			&ch.ChunkIndex,
			&ch.Text,
			&ch.FusionScore,
			&vectorSimilarity,
			&lexicalScore,
			&vectorRank,
			&lexicalRank,
		); err != nil {
			return nil, err
		}
		ch.SourceKind = "retrieved"
		ch.VectorRank = intPtrFromNull(vectorRank)
		ch.LexicalRank = intPtrFromNull(lexicalRank)
		ch.VectorSimilarity = floatPtrFromNull(vectorSimilarity)
		ch.LexicalScore = floatPtrFromNull(lexicalScore)
		chunks = append(chunks, ch)
	}
	return chunks, rows.Err()
}

func (s Searcher) RecentSeenChunks(ctx context.Context, sessionID string, limit int) ([]Chunk, error) {
	if limit <= 0 {
		limit = 3
	}
	rows, err := s.DB.QueryContext(ctx, `
		SELECT c.id,
		       c.document_id,
		       c.document_name,
		       c.source_path,
		       c.topic,
		       c.section,
		       c.chunk_index,
		       c.text,
		       0::float8 AS fusion_score,
		       NULL::float8 AS vector_similarity,
		       NULL::float8 AS lexical_score,
		       NULL::bigint AS vector_rank,
		       NULL::bigint AS lexical_rank,
		       row_number() OVER (ORDER BY sc.last_used_at DESC) AS memory_rank
		FROM chat_seen_chunks sc
		JOIN chunks c ON c.id = sc.chunk_id
		WHERE sc.session_id = $1
		ORDER BY sc.last_used_at DESC
		LIMIT $2
	`, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []Chunk
	for rows.Next() {
		var ch Chunk
		var vectorRank, lexicalRank, memoryRank sql.NullInt64
		var vectorSimilarity, lexicalScore sql.NullFloat64
		if err := rows.Scan(
			&ch.ID,
			&ch.DocumentID,
			&ch.DocumentName,
			&ch.SourcePath,
			&ch.Topic,
			&ch.Section,
			&ch.ChunkIndex,
			&ch.Text,
			&ch.FusionScore,
			&vectorSimilarity,
			&lexicalScore,
			&vectorRank,
			&lexicalRank,
			&memoryRank,
		); err != nil {
			return nil, err
		}
		ch.SourceKind = "memory"
		ch.VectorRank = intPtrFromNull(vectorRank)
		ch.LexicalRank = intPtrFromNull(lexicalRank)
		ch.MemoryRank = intPtrFromNull(memoryRank)
		ch.VectorSimilarity = floatPtrFromNull(vectorSimilarity)
		ch.LexicalScore = floatPtrFromNull(lexicalScore)
		chunks = append(chunks, ch)
	}
	return chunks, rows.Err()
}

func MergeChunks(fresh, memory []Chunk, max int) []Chunk {
	seen := map[string]bool{}
	out := make([]Chunk, 0, len(fresh)+len(memory))
	for _, ch := range fresh {
		if seen[ch.ID] {
			continue
		}
		seen[ch.ID] = true
		if ch.SourceKind == "" {
			ch.SourceKind = "retrieved"
		}
		out = append(out, ch)
	}
	for _, ch := range memory {
		if seen[ch.ID] {
			continue
		}
		seen[ch.ID] = true
		ch.SourceKind = "memory"
		out = append(out, ch)
	}
	if max > 0 && len(out) > max {
		return out[:max]
	}
	return out
}

func MustHaveChunks(chunks []Chunk) error {
	if len(chunks) == 0 {
		return fmt.Errorf("no chunks found; ingest documents first or broaden the query")
	}
	return nil
}

func intPtrFromNull(n sql.NullInt64) *int {
	if !n.Valid {
		return nil
	}
	v := int(n.Int64)
	return &v
}

func floatPtrFromNull(n sql.NullFloat64) *float64 {
	if !n.Valid {
		return nil
	}
	v := n.Float64
	return &v
}

const hybridSQL = `
WITH vector_matches AS (
    SELECT ranked.id,
           ranked.vector_rank,
           ranked.vector_similarity
    FROM (
        SELECT c.id,
               row_number() OVER (ORDER BY c.embedding <=> $1::vector) AS vector_rank,
               1.0 - (c.embedding <=> $1::vector) AS vector_similarity
        FROM chunks c
        ORDER BY c.embedding <=> $1::vector
        LIMIT $2
    ) ranked
),
lexical_base AS (
    SELECT c.id,
           ts_rank_cd(c.full_text, websearch_to_tsquery('english', $3)) AS lexical_score
    FROM chunks c
    WHERE c.full_text @@ websearch_to_tsquery('english', $3)
),
lexical_matches AS (
    SELECT ranked.id,
           ranked.lexical_rank,
           ranked.lexical_score
    FROM (
        SELECT lb.id,
               row_number() OVER (ORDER BY lb.lexical_score DESC) AS lexical_rank,
               lb.lexical_score
        FROM lexical_base lb
        ORDER BY lb.lexical_score DESC
        LIMIT $2
    ) ranked
),
all_matches (id, vector_rank, lexical_rank, vector_similarity, lexical_score) AS (
    SELECT vm.id,
           vm.vector_rank,
           NULL::bigint,
           vm.vector_similarity,
           NULL::float8
    FROM vector_matches vm
    UNION ALL
    SELECT lm.id,
           NULL::bigint,
           lm.lexical_rank,
           NULL::float8,
           lm.lexical_score
    FROM lexical_matches lm
),
combined AS (
    SELECT am.id,
           min(am.vector_rank) AS vector_rank,
           min(am.lexical_rank) AS lexical_rank,
           max(am.vector_similarity) AS vector_similarity,
           max(am.lexical_score) AS lexical_score
    FROM all_matches am
    GROUP BY am.id
),
ranked_results AS (
    SELECT c.id,
           c.document_id,
           c.document_name,
           c.source_path,
           c.topic,
           c.section,
           c.chunk_index,
           c.text,
           COALESCE(1.0 / ($4 + combined.vector_rank), 0.0) +
           COALESCE(1.0 / ($4 + combined.lexical_rank), 0.0) AS fusion_score,
           combined.vector_similarity,
           combined.lexical_score,
           combined.vector_rank,
           combined.lexical_rank
    FROM combined
    JOIN chunks c ON c.id = combined.id
)
SELECT id,
       document_id,
       document_name,
       source_path,
       topic,
       section,
       chunk_index,
       text,
       fusion_score,
       vector_similarity,
       lexical_score,
       vector_rank,
       lexical_rank
FROM ranked_results
ORDER BY fusion_score DESC
LIMIT $5
`
