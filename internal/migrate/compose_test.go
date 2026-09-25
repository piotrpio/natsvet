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
	"go/format"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
)

// edit is a test edit: replace [start, end) with text.
type edit struct {
	start, end int
	text       string
}

// logBatch applies sorted, non-overlapping edits to text and returns the
// new text with the batch as the simulation logs it.
func logBatch(text string, edits []edit) (string, []logEdit) {
	var b strings.Builder
	var batch []logEdit
	last := 0
	for _, e := range edits {
		b.WriteString(text[last:e.start])
		b.WriteString(e.text)
		last = e.end
		batch = append(batch, logEdit{start: e.start, end: e.end, n: len(e.text)})
	}
	b.WriteString(text[last:])
	return b.String(), batch
}

func TestComposeBatches(t *testing.T) {
	tests := []struct {
		name    string
		before  string
		batches [][]edit
		want    []edit
	}{
		{
			name:    "insertion inside an earlier insertion",
			before:  "abcdef",
			batches: [][]edit{{{3, 3, "XYZ"}}, {{4, 4, "Q"}}},
			want:    []edit{{3, 3, "XQYZ"}},
		},
		{
			name:    "rename inside inserted text",
			before:  "func f() {\n}\n",
			batches: [][]edit{{{11, 11, "\tjsNew := 1\n\t_ = jsNew\n"}}, {{12, 17, "js"}, {28, 33, "js"}}},
			want:    []edit{{11, 11, "\tjs := 1\n\t_ = js\n"}},
		},
		{
			name:    "deletion next to an insertion, and an edit elsewhere",
			before:  "aaa\nbbb\nccc\nddd\n",
			batches: [][]edit{{{4, 8, ""}, {12, 15, "D"}}, {{4, 4, "X\n"}}},
			want:    []edit{{4, 8, "X\n"}, {12, 15, "D"}},
		},
		{
			name:    "unchanged lines at the ends are left out",
			before:  "R1\nE\n",
			batches: [][]edit{{{5, 5, "S2\nE\n"}}, {{0, 5, ""}, {5, 6, "R"}}},
			want:    []edit{{0, 3, "R2\n"}},
		},
		{
			name:    "later batch deletes inserted text",
			before:  "abc",
			batches: [][]edit{{{1, 1, "123"}}, {{1, 4, ""}}},
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := tt.before
			var batches [][]logEdit
			for _, es := range tt.batches {
				var batch []logEdit
				text, batch = logBatch(text, es)
				batches = append(batches, batch)
			}
			got, err := composeBatches([]byte(tt.before), []byte(text), batches)
			if err != nil {
				t.Fatal(err)
			}
			var ge []edit
			for _, it := range got {
				ge = append(ge, edit{it.start, it.end, it.text})
			}
			if !slices.Equal(ge, tt.want) {
				t.Errorf("composed %+v, want %+v", ge, tt.want)
			}
		})
	}
}

// TestComposeBatchesRandom composes random batches and checks that the
// composed edits turn the text before them into the text after them.
func TestComposeBatchesRandom(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	const alphabet = "ab\n "
	randText := func(n int) string {
		b := make([]byte, n)
		for i := range b {
			b[i] = alphabet[r.IntN(len(alphabet))]
		}
		return string(b)
	}
	for range 2000 {
		before := randText(r.IntN(20))
		text := before
		var batches [][]logEdit
		for range 1 + r.IntN(4) {
			var edits []edit
			pos := 0
			for pos <= len(text) && r.IntN(3) > 0 {
				start := pos + r.IntN(len(text)-pos+1)
				end := start + r.IntN(min(3, len(text)-start)+1)
				edits = append(edits, edit{start, end, randText(r.IntN(4))})
				pos = end + 1
			}
			var batch []logEdit
			text, batch = logBatch(text, edits)
			batches = append(batches, batch)
		}
		got, err := composeBatches([]byte(before), []byte(text), batches)
		if err != nil {
			t.Fatalf("before %q, batches %+v: %v", before, batches, err)
		}
		if s := string(applyText([]byte(before), got)); s != text {
			t.Fatalf("before %q, batches %+v: composed edits give %q, want %q", before, batches, s, text)
		}
		for i := 1; i < len(got); i++ {
			if got[i].start <= got[i-1].end {
				t.Fatalf("before %q, batches %+v: composed edits touch or overlap: %+v", before, batches, got)
			}
		}
	}
}

func TestSpanMarksSurviveInnerEdits(t *testing.T) {
	const text = "a\n\t_ = js\n\t_ = jsNew\nb\n"
	span := mark{off: 2, n: 19, kind: markSpan, key: "placeholder:js"}
	sib := mark{off: 14, n: 5, kind: markSibling, key: "js"}
	tests := []struct {
		name  string
		edits []intent
		want  []mark
	}{
		{"deletion inside", []intent{{start: 5, end: 6}}, []mark{{off: 2, n: 18, kind: markSpan, key: "placeholder:js"}, {off: 13, n: 5, kind: markSibling, key: "js"}}},
		{"insertion inside", []intent{{start: 5, end: 5, text: "  "}}, []mark{{off: 2, n: 21, kind: markSpan, key: "placeholder:js"}, {off: 16, n: 5, kind: markSibling, key: "js"}}},
		{"edit across the start", []intent{{start: 0, end: 4, text: "x"}}, []mark{{off: 11, n: 5, kind: markSibling, key: "js"}}},
		{"whole span replaced", []intent{{start: 2, end: 21}}, nil},
		{"sibling renamed", []intent{{start: 14, end: 19, text: "js"}}, []mark{{off: 2, n: 16, kind: markSpan, key: "placeholder:js"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sf := &simFile{text: []byte(text), marks: []mark{span, sib}}
			sf.applyBatch(tt.edits)
			if !slices.Equal(sf.marks, tt.want) {
				t.Errorf("marks %+v, want %+v", sf.marks, tt.want)
			}
		})
	}
}

func TestFormatEdits(t *testing.T) {
	tests := []struct {
		name, src string
		changes   bool
	}{
		{"struct alignment", "package p\n\ntype S struct {\n\tNC   int\n\tJS   int\n\tJSNew string\n\tName string\n}\n", true},
		{"expression spacing", "package p\n\nvar d = f(2 * 3)\n\nfunc f(int) int { return 0 }\n", true},
		{"composite literal alignment", "package p\n\nvar c = struct{ A, Bcd int }{\n\tA: 1,\n\tBcd: 2,\n}\n", true},
		{"already clean", "package p\n\nvar x = 1\n", false},
		{"imports out of order", "package p\n\nimport (\n\t\"os\"\n\t\"fmt\"\n)\n\nvar _ = fmt.Sprint\nvar _ = os.Exit\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			edits := formatEdits([]byte(tt.src))
			if !tt.changes {
				if len(edits) > 0 {
					t.Errorf("edits %+v, want none", edits)
				}
				return
			}
			want, err := format.Source([]byte(tt.src))
			if err != nil {
				t.Fatal(err)
			}
			if got := applyText([]byte(tt.src), edits); string(got) != string(want) {
				t.Errorf("formatted:\n%s\nwant:\n%s", got, want)
			}
			for _, e := range edits {
				if strings.TrimSpace(tt.src[e.start:e.end]) != "" || strings.TrimSpace(e.text) != "" {
					t.Errorf("edit %+v changes more than whitespace", e)
				}
			}
		})
	}
}
