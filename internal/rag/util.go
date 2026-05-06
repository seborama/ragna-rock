// SPDX-License-Identifier: MPL-2.0

/*
 * This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at https://mozilla.org/MPL/2.0/.
 */

package rag

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
)

func NewID(prefix string) string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(b[:])
}

func StableID(prefix, value string) string {
	h := sha256.Sum256([]byte(value))
	return prefix + "_" + hex.EncodeToString(h[:12])
}

func VectorLiteral(v []float64) string {
	parts := make([]string, len(v))
	for i, x := range v {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			x = 0
		}
		parts[i] = fmt.Sprintf("%.8f", x)
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func TrimForPrompt(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
