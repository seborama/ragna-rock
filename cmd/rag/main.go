// SPDX-License-Identifier: MPL-2.0

/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/seborama/ragna-rock/internal/chat"
	"github.com/seborama/ragna-rock/internal/config"
	"github.com/seborama/ragna-rock/internal/db"
	"github.com/seborama/ragna-rock/internal/ingest"
	"github.com/seborama/ragna-rock/internal/ollama"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	cfg := config.Load()
	database, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()

	ollamaClient := ollama.New(cfg.OllamaURL)

	switch os.Args[1] {
	case "migrate":
		if err := db.Migrate(ctx, database, "migrations"); err != nil {
			log.Fatal(err)
		}
		fmt.Println("migrations applied")
	case "ingest":
		if len(os.Args) < 3 {
			log.Fatal("usage: rag ingest <path>")
		}
		stats, err := ingest.Service{DB: database, Ollama: ollamaClient, EmbedModel: cfg.EmbedModel}.IngestPath(ctx, os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("ingested %d document(s), %d chunk(s)\n", stats.Documents, stats.Chunks)
	case "ask":
		question := strings.Join(os.Args[2:], " ")
		if question == "" {
			log.Fatal("usage: rag ask <question>")
		}
		res, err := chat.Service{DB: database, Cfg: cfg, Ollama: ollamaClient}.OneShot(ctx, question)
		if err != nil {
			log.Fatal(err)
		}
		printResponse(res, false)
	case "chat":
		fs := flag.NewFlagSet("chat", flag.ExitOnError)
		sessionID := fs.String("session", "default", "chat session id")
		_ = fs.Parse(os.Args[2:])
		message := strings.Join(fs.Args(), " ")
		if message == "" {
			log.Fatal("usage: rag chat --session <id> <message>")
		}
		res, err := chat.Service{DB: database, Cfg: cfg, Ollama: ollamaClient}.Chat(ctx, *sessionID, message)
		if err != nil {
			log.Fatal(err)
		}
		printResponse(res, true)
	default:
		usage()
		os.Exit(2)
	}
}

func printResponse(res chat.Response, showSession bool) {
	if showSession {
		fmt.Printf("Session: %s\n", res.SessionID)
	}
	fmt.Printf("Retrieval query: %s\n\n", res.RewrittenQuery)
	fmt.Println(res.Answer)
	fmt.Println("\nSources:")
	for i, ch := range res.Chunks {
		fmt.Printf("[%d] %s — section=%q chunk=%d kind=%s fusion_score=%.5f vector_similarity=%s vector_rank=%s lexical_rank=%s memory_rank=%s path=%s\n",
			i+1,
			ch.DocumentName,
			ch.Section,
			ch.ChunkIndex,
			valueOr(ch.SourceKind, "retrieved"),
			ch.FusionScore,
			formatFloatPtr(ch.VectorSimilarity),
			formatIntPtr(ch.VectorRank),
			formatIntPtr(ch.LexicalRank),
			formatIntPtr(ch.MemoryRank),
			ch.SourcePath,
		)
	}
}

func formatIntPtr(v *int) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *v)
}

func formatFloatPtr(v *float64) string {
	if v == nil {
		return "-"
	}
	return fmt.Sprintf("%.5f", *v)
}

func valueOr(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

func usage() {
	fmt.Println(`usage:
  rag migrate
  rag ingest <path-to-.txt-or-.md-file-or-directory>
  rag ask <question>
  rag chat --session <id> <message>

examples:
  go run ./cmd/rag migrate
  go run ./cmd/rag ingest ./docs/sample
  go run ./cmd/rag chat --session ddia "Let's talk about partitioning"
  go run ./cmd/rag chat --session ddia "How does that apply to Kafka partitioning keys?"`)
}
