// SPDX-License-Identifier: MPL-2.0

/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package config

import (
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL string
	OllamaURL   string
	EmbedModel  string
	ChatModel   string
	TopK        int
	Candidates  int
	RRFK        int
}

func Load() Config {
	return Config{
		DatabaseURL: env("DATABASE_URL", "postgres://rag:rag@localhost:5432/rag?sslmode=disable"),
		OllamaURL:   env("OLLAMA_URL", "http://localhost:11434"),
		EmbedModel:  env("OLLAMA_EMBED_MODEL", "bge-m3"),
		ChatModel:   env("OLLAMA_CHAT_MODEL", "llama3.1"),
		TopK:        envInt("RAG_TOP_K", 8),
		Candidates:  envInt("RAG_CANDIDATES", 24),
		RRFK:        envInt("RAG_RRF_K", 60),
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
