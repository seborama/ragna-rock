# SPDX-License-Identifier: MPL-2.0
#
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v. 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at https://mozilla.org/MPL/2.0/.

.PHONY: up down migrate ingest sample ask chat build

up:
	docker compose up -d

down:
	docker compose down

build:
	go build ./cmd/rag

migrate:
	go run ./cmd/rag migrate

sample:
	go run ./cmd/rag ingest ./docs/sample

ask:
	go run ./cmd/rag ask "Explain partitioning and why key choice matters"

chat:
	go run ./cmd/rag chat --session ddia "Let's talk about partitioning"
