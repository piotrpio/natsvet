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
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"
)

// intent is an edit against the original file: replace [start, end) with
// text, or insert text at start when start == end.
type intent struct {
	file       *srcFile
	start, end int // byte offsets in the original file
	text       string
	// marks are sibling identifiers inside text, renamed in the last step.
	marks []mark
}

// markKind says what a mark tracks.
type markKind int

const (
	// markSibling is a sibling identifier, renamed in the last step.
	markSibling markKind = iota
	// markSpan is inserted text a later step deletes or replaces.
	markSpan
)

// mark tracks a range of inserted text through later steps: off and n
// are its offset in the intent's text and its length.
type mark struct {
	off, n     int
	kind       markKind
	name, orig string // sibling name and the legacy name it takes at the end
	key        string // the handle a sibling belongs to, or a span's key
}

// builder assembles replacement text and records where it places sibling
// identifiers and spans.
type builder struct {
	b     strings.Builder
	marks []mark
}

func (b *builder) add(s ...string) *builder {
	for _, x := range s {
		b.b.WriteString(x)
	}
	return b
}

// sib writes a handle's sibling identifier and records it for the rename
// step.
func (b *builder) sib(h *handle) *builder {
	b.marks = append(b.marks, mark{off: b.b.Len(), n: len(h.sibling), kind: markSibling, name: h.sibling, orig: h.ident.Name, key: string(h.key)})
	b.b.WriteString(h.sibling)
	return b
}

// span records the text written since start as a span with key.
func (b *builder) span(start int, key string) {
	b.marks = append(b.marks, mark{off: start, n: b.b.Len() - start, kind: markSpan, key: key})
}

func (b *builder) len() int { return b.b.Len() }

// embed writes text that carries its own marks.
func (b *builder) embed(text string, marks []mark) *builder {
	for _, m := range marks {
		m.off += b.b.Len()
		b.marks = append(b.marks, m)
	}
	b.b.WriteString(text)
	return b
}

func (b *builder) String() string { return b.b.String() }

// at returns an intent replacing [start, end) of f with the built text.
func (b *builder) at(f *srcFile, start, end int) intent {
	return intent{file: f, start: start, end: end, text: b.b.String(), marks: b.marks}
}

// text returns the source text of a node.
func (prog *program) text(n ast.Node) string {
	f := prog.fileOf(n.Pos())
	return string(f.src[prog.offset(n.Pos()):prog.offset(n.End())])
}

// replace returns an intent replacing node n with text.
func (prog *program) replace(n ast.Node, text string) intent {
	return intent{file: prog.fileOf(n.Pos()), start: prog.offset(n.Pos()), end: prog.offset(n.End()), text: text}
}

// insertAt returns an intent inserting text at pos.
func (prog *program) insertAt(pos token.Pos, text string) intent {
	o := prog.offset(pos)
	return intent{file: prog.fileOf(pos), start: o, end: o, text: text}
}

// indentOf returns the leading whitespace of the line containing pos.
func (prog *program) indentOf(pos token.Pos) string {
	f := prog.fileOf(pos)
	o := prog.offset(pos)
	start := o
	for start > 0 && f.src[start-1] != '\n' {
		start--
	}
	end := start
	for end < len(f.src) && (f.src[end] == ' ' || f.src[end] == '\t') {
		end++
	}
	return string(f.src[start:end])
}

// lineSpan returns the offsets of the full lines holding [pos, end),
// including the trailing newline, so deleting them leaves no blank line.
func (prog *program) lineSpan(pos, end token.Pos) (int, int) {
	f := prog.fileOf(pos)
	s, e := prog.offset(pos), prog.offset(end)
	for s > 0 && f.src[s-1] != '\n' {
		s--
	}
	for e < len(f.src) && f.src[e] != '\n' {
		e++
	}
	if e < len(f.src) {
		e++
	}
	return s, e
}

// applyWithin returns the text of n with the intents that fall inside it
// applied; intents must not overlap.
func (prog *program) applyWithin(n ast.Node, intents []intent) string {
	base := prog.offset(n.Pos())
	src := prog.text(n)
	var in []intent
	for _, it := range intents {
		if it.start >= base && it.end <= base+len(src) {
			in = append(in, it)
		}
	}
	slices.SortFunc(in, func(a, b intent) int { return b.start - a.start })
	for _, it := range in {
		src = src[:it.start-base] + it.text + src[it.end-base:]
	}
	return src
}

// applyWithinMarks is applyWithin that also returns the marks of the
// applied intents, as offsets in the returned text.
func (prog *program) applyWithinMarks(n ast.Node, intents []intent) (string, []mark) {
	base := prog.offset(n.Pos())
	src := prog.text(n)
	var in []intent
	for _, it := range intents {
		if it.start >= base && it.end <= base+len(src) {
			in = append(in, it)
		}
	}
	slices.SortFunc(in, func(a, b intent) int { return a.start - b.start })
	var b strings.Builder
	var marks []mark
	last := 0
	for _, it := range in {
		b.WriteString(src[last : it.start-base])
		for _, m := range it.marks {
			m.off += b.Len()
			marks = append(marks, m)
		}
		b.WriteString(it.text)
		last = it.end - base
	}
	b.WriteString(src[last:])
	return b.String(), marks
}

// reindentMarks adds prefix after every newline of text, moving the marks
// with it, unless text holds a raw string literal.
func reindentMarks(text string, marks []mark, prefix string) (string, []mark) {
	if strings.Contains(text, "`") {
		return text, marks
	}
	out := make([]mark, len(marks))
	for i, m := range marks {
		m.off += strings.Count(text[:m.off], "\n") * len(prefix)
		out[i] = m
	}
	return strings.ReplaceAll(text, "\n", "\n"+prefix), out
}

// ctxAt returns the context expression a jetstream call at pos receives:
// a context.Context variable in scope, else the argument of a
// nats.Context option among opts, else context.Background().
func (prog *program) ctxAt(f *srcFile, pos token.Pos, opts []ast.Expr) (expr string, background bool) {
	scope := f.pkg.Types.Scope().Innermost(pos)
	for s := scope; s != nil && s != f.pkg.Types.Scope() && s != types.Universe; s = s.Parent() {
		var best types.Object
		for _, name := range s.Names() {
			obj := s.Lookup(name)
			v, ok := obj.(*types.Var)
			if !ok || name == "_" || !isContext(v.Type()) || (obj.Pos().IsValid() && obj.Pos() > pos) {
				continue
			}
			if best == nil || obj.Pos() > best.Pos() {
				best = obj
			}
		}
		if best != nil {
			return best.Name(), false
		}
	}
	info := f.info()
	for _, o := range opts {
		call, ok := ast.Unparen(o).(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			continue
		}
		if sym, ok := calleeKey(info, call); ok && sym == "Context" {
			return prog.text(call.Args[0]), false
		}
	}
	return "context.Background()", true
}

func isContext(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	return ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "context" && n.Obj().Name() == "Context"
}
