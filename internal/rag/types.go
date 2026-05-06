// SPDX-License-Identifier: MPL-2.0

/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package rag

import "time"

type Chunk struct {
	ID           string
	DocumentID   string
	DocumentName string
	SourcePath   string
	Topic        string
	Section      string
	ChunkIndex   int
	Text         string

	// FusionScore is the Reciprocal Rank Fusion score used for ordering hybrid
	// retrieval results. It is not a confidence score.
	FusionScore float64

	// Optional debug signals. They are nil when the chunk did not come from that
	// retrieval path, or when the chunk came from session memory rather than a
	// fresh retrieval pass.
	VectorRank       *int
	LexicalRank      *int
	MemoryRank       *int
	VectorSimilarity *float64
	LexicalScore     *float64

	// SourceKind is either "retrieved" or "memory".
	SourceKind string
}

type ChatSession struct {
	ID           string
	Title        string
	Summary      string
	ActiveTopics []string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ChatTurn struct {
	ID        string
	SessionID string
	Role      string
	Content   string
	CreatedAt time.Time
}
