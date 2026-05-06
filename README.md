# ragna-rock

<p align="center">
  <img src="./assets/ragna-rock.png" alt="ragna-rock mascot" width="320">
</p>

A small local RAG playground forged from chunks, embeddings, hybrid search and stubborn curiosity.

A Go pgvector RAG with Ollama, BGE, hybrid search, and conversational memory.

This is a local “chat with my documents” workbench in Go.

It builds on a simple RAG pipeline:

```text
documents -> chunks -> BGE embeddings via Ollama -> pgvector + PostgreSQL full-text search -> LLM answer
```

This version adds a thin conversational layer:

```text
latest user message
+ session summary
+ recent turns
+ active topics
  -> rewrite into a standalone retrieval query
  -> hybrid retrieval: pgvector + PostgreSQL full-text search
  -> add a few previously seen chunks from this session
  -> answer with citations
  -> update session summary, active topics, and seen chunks
```

It is not trying to be Glean. It is the smaller, local equivalent of the useful bit: a context layer around retrieval.

## Requirements

- Go 1.26+
- Docker, for Postgres + pgvector
- Ollama running locally
- Ollama models pulled locally, for example:

```bash
ollama pull bge-m3
ollama pull llama3.1
```

The default migration uses `vector(1024)`, which matches `bge-m3`. If you use a different embedding model, update `migrations/001_init.sql` before running migrations.

## Start Postgres

```bash
docker compose up -d
```

## Configure

Defaults are already local-friendly:

```bash
export DATABASE_URL='postgres://rag:rag@localhost:5432/rag?sslmode=disable'
export OLLAMA_URL='http://localhost:11434'
export OLLAMA_EMBED_MODEL='bge-m3'
export OLLAMA_CHAT_MODEL='llama3.1'
```

## Migrate

```bash
go run ./cmd/rag migrate
```

## Ingest sample docs

```bash
go run ./cmd/rag ingest ./docs/sample
```

The ingester currently supports `.txt` and `.md`. For PDFs, convert them to text first, then ingest the resulting files. That keeps the example focused on RAG rather than PDF extraction quality.

## One-shot RAG question

```bash
go run ./cmd/rag ask "Explain partitioning and why key choice matters"
```

This does normal hybrid RAG without session memory.

## Conversational RAG

Start a named session:

```bash
go run ./cmd/rag chat --session ddia "Let's talk about partitioning"
```

Then ask a follow-up:

```bash
go run ./cmd/rag chat --session ddia "How does that apply to Kafka partitioning keys?"
```

The second command should rewrite the follow-up into a standalone retrieval query using the session summary and recent turns, retrieve fresh chunks, and also make recent session chunks available as memory.

## What changed from plain RAG?

Plain RAG normally does this:

```text
question -> embed -> retrieve -> answer
```

Conversational RAG needs more state:

| Concern | Why it matters |
|---|---|
| `chat_sessions.summary` | Keeps the durable thread of the conversation without stuffing every old turn into the prompt. |
| `chat_sessions.active_topics` | Helps follow-up retrieval stay on-topic. |
| `chat_turns` | Keeps recent back-and-forth available for pronoun/reference resolution. |
| `chat_seen_chunks` | Lets the system re-include earlier retrieved context when follow-ups depend on it. |
| query rewriting | Turns “how does that apply to Kafka?” into a standalone retrieval query. |

## Hybrid search

The search query combines:

- pgvector cosine similarity over embeddings
- PostgreSQL full-text search using `websearch_to_tsquery` and `ts_rank_cd`
- reciprocal rank fusion to merge semantic and lexical matches
- debug output separates `fusion_score` from `vector_similarity`, because RRF scores are ranking signals rather than confidence scores
- follow-up turns keep previous chunks as `kind=memory` context instead of treating them as fresh retrieval hits

This is “BM25-ish” in the PostgreSQL full-text sense, not a full Lucene/Elasticsearch BM25 implementation. It is good enough for a compact local workbench.

## Important limitations

- The answer can still drift if the prompt allows too much general knowledge.
- The query rewriter is LLM-based, so it can occasionally over-expand or under-expand a query.
- There is no permission model.
- There is no incremental re-indexing beyond replacing chunks for a file path during ingestion.
- There is no PDF parsing in this example.
- There is no prompt-injection defence beyond basic instruction hierarchy in the answer prompt.

The next hardening steps would be:

1. Add page/section metadata from the PDF conversion stage.
2. Add stricter source-grounding checks.
3. Store document hashes and skip unchanged files.
4. Add a proper reranker.
5. Add a prompt-injection scan or defensive context treatment for untrusted documents.
6. Add tests around query rewriting and retrieval behaviour.

## License

This project is licensed under the Mozilla Public License 2.0 (MPL-2.0).

See [LICENSE](./LICENSE) for details.

### What this means

You are free to use this project in commercial and non-commercial settings.

If you modify MPL-covered source files and distribute those changes, the modified files must remain available under MPL-2.0.

You can build proprietary software around this project without making your own separate files open source.

### Why MPL-2.0?

MPL-2.0 keeps improvements to the core code open, while still allowing broad reuse.
