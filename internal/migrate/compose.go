// Copyright 2026 Synadia Communications Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package migrate

import (
	"bytes"
	"fmt"
	"go/format"
	"go/scanner"
	"go/token"
	"slices"
	"strings"
)

// composeBatches turns batches of edits, each in the coordinates of the
// text the previous batch left, into one batch in the coordinates of
// before. Every edit is mapped back to before, an edit inside text an
// earlier batch inserted covering the range that text replaced; the ranges
// are merged where they overlap or touch, and each merged range takes its
// text from after. The result is checked to turn before into after.
func composeBatches(before, after []byte, batches [][]logEdit) ([]intent, error) {
	type span struct{ start, end int }
	var spans []span
	for bi, batch := range batches {
		for _, e := range batch {
			s, en := e.start, e.end
			for j := bi - 1; j >= 0; j-- {
				s, en = backPos(batches[j], s, true), backPos(batches[j], en, false)
			}
			spans = append(spans, span{s, en})
		}
	}
	slices.SortFunc(spans, func(a, b span) int {
		if a.start != b.start {
			return a.start - b.start
		}
		return a.end - b.end
	})
	var merged []span
	for _, s := range spans {
		if n := len(merged); n > 0 && s.start <= merged[n-1].end {
			merged[n-1].end = max(merged[n-1].end, s.end)
			continue
		}
		merged = append(merged, s)
	}
	var out []intent
	for _, s := range merged {
		a, b := s.start, s.end
		for _, batch := range batches {
			a, b = fwdPos(batch, a, false), fwdPos(batch, b, true)
		}
		if a > b || b > len(after) {
			return nil, fmt.Errorf("composed range [%d, %d) maps outside the result", s.start, s.end)
		}
		old, text := string(before[s.start:s.end]), string(after[a:b])
		if text == old {
			continue
		}
		// Leave out whole lines the range keeps at either end.
		p := 0
		for p < len(old) && p < len(text) && old[p] == text[p] {
			p++
		}
		p = strings.LastIndexByte(old[:p], '\n') + 1
		q := 0
		for q < len(old)-p && q < len(text)-p && old[len(old)-1-q] == text[len(text)-1-q] {
			q++
		}
		t := len(old) - q
		for t < len(old) && (t == 0 || old[t-1] != '\n') {
			t++
		}
		out = append(out, intent{start: s.start + p, end: s.start + t, text: text[p : len(text)-(len(old)-t)]})
	}
	if got := applyText(before, out); !bytes.Equal(got, after) {
		return nil, fmt.Errorf("composed edits do not reproduce the batches")
	}
	return out, nil
}

// backPos maps offset p of the text after batch to the text before it. An
// offset in or next to text the batch inserted maps to the start (lo) or
// end of the range that text replaced.
func backPos(batch []logEdit, p int, lo bool) int {
	shift := 0
	for _, e := range batch {
		ps := e.start + shift
		pe := ps + e.n
		if p < ps {
			break
		}
		if p <= pe {
			if lo {
				return e.start
			}
			return e.end
		}
		shift += e.n - (e.end - e.start)
	}
	return p - shift
}

// fwdPos maps offset p of the text before batch to the text after it; text
// inserted at p counts as before it when after is set.
func fwdPos(batch []logEdit, p int, after bool) int {
	delta := 0
	for _, e := range batch {
		switch {
		case e.start == e.end:
			if e.start < p || (e.start == p && after) {
				delta += e.n
			}
		case e.end <= p:
			delta += e.n - (e.end - e.start)
		}
	}
	return p + delta
}

// formatEdits returns the whitespace edits that turn src into what gofmt
// makes of it: one minimal insertion, deletion or replacement per gap
// between tokens that differs. It returns none when gofmt fails or changes
// anything but whitespace (reordered imports, reindented comments).
func formatEdits(src []byte) []intent {
	formatted, err := format.Source(src)
	if err != nil || bytes.Equal(formatted, src) {
		return nil
	}
	a, b := scanTokens(src), scanTokens(formatted)
	if len(a) != len(b) {
		return nil
	}
	var out []intent
	prevA, prevB := 0, 0
	for i := range len(a) + 1 {
		endA, endB := len(src), len(formatted)
		if i < len(a) {
			if a[i].text != b[i].text {
				return nil
			}
			endA, endB = a[i].off, b[i].off
		}
		ga, gb := src[prevA:endA], formatted[prevB:endB]
		if !bytes.Equal(ga, gb) {
			p := 0
			for p < len(ga) && p < len(gb) && ga[p] == gb[p] {
				p++
			}
			s := 0
			for s < len(ga)-p && s < len(gb)-p && ga[len(ga)-1-s] == gb[len(gb)-1-s] {
				s++
			}
			out = append(out, intent{start: prevA + p, end: endA - s, text: string(gb[p : len(gb)-s])})
		}
		if i < len(a) {
			prevA, prevB = a[i].off+len(a[i].text), b[i].off+len(b[i].text)
		}
	}
	return out
}

// srcToken is a token of a source file with its text and offset.
type srcToken struct {
	off  int
	text string
}

// scanTokens returns the tokens of src, comments included and inserted
// semicolons left out.
func scanTokens(src []byte) []srcToken {
	fset := token.NewFileSet()
	f := fset.AddFile("", -1, len(src))
	var s scanner.Scanner
	s.Init(f, src, nil, scanner.ScanComments)
	var out []srcToken
	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			return out
		}
		if tok == token.SEMICOLON && lit == "\n" {
			continue
		}
		text := lit
		if text == "" {
			text = tok.String()
		}
		out = append(out, srcToken{off: f.Offset(pos), text: text})
	}
}
