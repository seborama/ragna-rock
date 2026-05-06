// SPDX-License-Identifier: MPL-2.0

/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package chunking

import (
	"regexp"
	"strings"
)

type Chunk struct {
	Index   int
	Section string
	Text    string
}

type Chunker struct {
	MaxChars int
	Overlap  int
}

func Default() Chunker {
	return Chunker{MaxChars: 1800, Overlap: 250}
}

func (c Chunker) Split(text string) []Chunk {
	text = normalise(text)
	if text == "" {
		return nil
	}

	paras := strings.Split(text, "\n\n")
	var chunks []Chunk
	var current strings.Builder
	section := ""
	idx := 0

	flush := func() {
		body := strings.TrimSpace(current.String())
		if body == "" {
			return
		}
		chunks = append(chunks, Chunk{Index: idx, Section: section, Text: body})
		idx++
		if c.Overlap > 0 && len(body) > c.Overlap {
			current.Reset()
			current.WriteString(body[len(body)-c.Overlap:])
			current.WriteString("\n\n")
		} else {
			current.Reset()
		}
	}

	for _, para := range paras {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		if isHeading(para) {
			section = strings.Trim(para, "# ")
		}
		if current.Len()+len(para)+2 > c.MaxChars {
			flush()
		}
		current.WriteString(para)
		current.WriteString("\n\n")
	}
	flush()
	return chunks
}

var whitespace = regexp.MustCompile(`[ \t]+`)

func normalise(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = whitespace.ReplaceAllString(s, " ")
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

func isHeading(s string) bool {
	if strings.HasPrefix(s, "#") && len(s) < 120 {
		return true
	}
	return len(s) < 90 && !strings.Contains(s, ".") && !strings.Contains(s, ",")
}
