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

	"github.com/seborama/ragna-rock/internal/config"
	"github.com/seborama/ragna-rock/internal/ollama"
	"github.com/seborama/ragna-rock/internal/rag"
)

type Service struct {
	DB     *sql.DB
	Cfg    config.Config
	Ollama *ollama.Client
}

type Response struct {
	SessionID      string
	RewrittenQuery string
	Answer         string
	Chunks         []rag.Chunk
}

func (s Service) OneShot(ctx context.Context, userMessage string) (Response, error) {
	emb, err := s.Ollama.Embedding(ctx, s.Cfg.EmbedModel, userMessage)
	if err != nil {
		return Response{}, err
	}
	chunks, err := rag.Searcher{DB: s.DB}.HybridSearch(ctx, userMessage, emb, rag.SearchOptions{
		Candidates: s.Cfg.Candidates,
		TopK:       s.Cfg.TopK,
		RRFK:       s.Cfg.RRFK,
	})
	if err != nil {
		return Response{}, err
	}
	if err := rag.MustHaveChunks(chunks); err != nil {
		return Response{}, err
	}
	answer, err := s.Ollama.Generate(ctx, s.Cfg.ChatModel, OneShotAnswerPrompt(userMessage, chunks))
	if err != nil {
		return Response{}, err
	}
	return Response{RewrittenQuery: userMessage, Answer: answer, Chunks: chunks}, nil
}

func (s Service) Chat(ctx context.Context, sessionID, userMessage string) (Response, error) {
	store := Store{DB: s.DB}
	session, err := store.GetOrCreateSession(ctx, sessionID)
	if err != nil {
		return Response{}, err
	}
	recentBefore, err := store.RecentTurns(ctx, session.ID, 6)
	if err != nil {
		return Response{}, err
	}

	rewritten, err := s.Ollama.Generate(ctx, s.Cfg.ChatModel, RewritePrompt(session, recentBefore, userMessage))
	if err != nil {
		return Response{}, fmt.Errorf("rewrite query: %w", err)
	}
	rewritten = cleanSingleLine(rewritten)
	if rewritten == "" {
		rewritten = userMessage
	}

	emb, err := s.Ollama.Embedding(ctx, s.Cfg.EmbedModel, rewritten)
	if err != nil {
		return Response{}, err
	}
	searcher := rag.Searcher{DB: s.DB}
	fresh, err := searcher.HybridSearch(ctx, rewritten, emb, rag.SearchOptions{
		Candidates: s.Cfg.Candidates,
		TopK:       s.Cfg.TopK,
		RRFK:       s.Cfg.RRFK,
	})
	if err != nil {
		return Response{}, err
	}
	memory, err := searcher.RecentSeenChunks(ctx, session.ID, 3)
	if err != nil {
		return Response{}, err
	}
	chunks := rag.MergeChunks(fresh, memory, s.Cfg.TopK+3)
	if err := rag.MustHaveChunks(chunks); err != nil {
		return Response{}, err
	}

	if err := store.AddTurn(ctx, session.ID, "user", userMessage); err != nil {
		return Response{}, err
	}
	recentAfterUser, err := store.RecentTurns(ctx, session.ID, 8)
	if err != nil {
		return Response{}, err
	}

	answer, err := s.Ollama.Generate(ctx, s.Cfg.ChatModel, AnswerPrompt(session, recentAfterUser, userMessage, rewritten, chunks))
	if err != nil {
		return Response{}, err
	}
	if err := store.AddTurn(ctx, session.ID, "assistant", answer); err != nil {
		return Response{}, err
	}
	if err := store.MarkSeenChunks(ctx, session.ID, chunks); err != nil {
		return Response{}, err
	}

	// Memory update failure should not hide the answer. It is useful, but not critical.
	if err := s.updateMemory(ctx, store, session, answer); err != nil {
		fmt.Printf("warning: could not update session memory: %v\n", err)
	}

	return Response{SessionID: session.ID, RewrittenQuery: rewritten, Answer: answer, Chunks: chunks}, nil
}

func (s Service) updateMemory(ctx context.Context, store Store, session rag.ChatSession, latestAnswer string) error {
	recent, err := store.RecentTurns(ctx, session.ID, 10)
	if err != nil {
		return err
	}
	raw, err := s.Ollama.Generate(ctx, s.Cfg.ChatModel, SummarisePrompt(session.Summary, session.ActiveTopics, recent, latestAnswer))
	if err != nil {
		return err
	}
	var parsed struct {
		Summary      string   `json:"summary"`
		ActiveTopics []string `json:"active_topics"`
	}
	if err := json.Unmarshal([]byte(extractJSONObject(raw)), &parsed); err != nil {
		return err
	}
	return store.UpdateSessionMemory(ctx, session.ID, parsed.Summary, parsed.ActiveTopics)
}

func cleanSingleLine(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`\"'")
	lines := strings.Split(s, "\n")
	return strings.TrimSpace(lines[0])
}

func extractJSONObject(s string) string {
	s = strings.TrimSpace(s)
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
