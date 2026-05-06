// SPDX-License-Identifier: MPL-2.0

/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func (c *Client) Embedding(ctx context.Context, model, text string) ([]float64, error) {
	body := map[string]any{"model": model, "prompt": text}
	var out struct {
		Embedding []float64 `json:"embedding"`
	}
	if err := c.post(ctx, "/api/embeddings", body, &out); err != nil {
		return nil, err
	}
	if len(out.Embedding) == 0 {
		return nil, fmt.Errorf("ollama returned an empty embedding")
	}
	return out.Embedding, nil
}

func (c *Client) Generate(ctx context.Context, model, prompt string) (string, error) {
	body := map[string]any{
		"model":  model,
		"prompt": prompt,
		"stream": false,
		"options": map[string]any{
			"temperature": 0.2,
		},
	}
	var out struct {
		Response string `json:"response"`
		Done     bool   `json:"done"`
	}
	if err := c.post(ctx, "/api/generate", body, &out); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.Response), nil
}

func (c *Client) post(ctx context.Context, path string, in any, out any) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("ollama %s returned %d: %s", path, res.StatusCode, string(resBody))
	}
	if err := json.Unmarshal(resBody, out); err != nil {
		return fmt.Errorf("decode ollama response: %w; body=%s", err, string(resBody))
	}
	return nil
}
