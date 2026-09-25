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
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"slices"
	"strconv"
	"strings"
)

// knownNames are the package names of imports a step may add or drop.
var knownNames = map[string]string{
	natsModule: "nats",
	jsPath:     "jetstream",
	"context":  "context",
}

// importName returns the name an import spec binds.
func importName(spec *ast.ImportSpec) string {
	if spec.Name != nil {
		return spec.Name.Name
	}
	p, _ := strconv.Unquote(spec.Path.Value)
	if n, ok := knownNames[p]; ok {
		return n
	}
	base := path.Base(p)
	if i := strings.IndexAny(base, ".-"); i > 0 {
		base = base[:i]
	}
	if strings.HasPrefix(base, "v") && len(base) > 1 && strings.Trim(base[1:], "0123456789") == "" {
		base = path.Base(path.Dir(p))
	}
	return base
}

// applyText applies sorted, non-overlapping intents to text.
func applyText(text []byte, its []intent) []byte {
	var b strings.Builder
	last := 0
	for _, it := range its {
		b.Write(text[last:it.start])
		b.WriteString(it.text)
		last = it.end
	}
	b.Write(text[last:])
	return []byte(b.String())
}

// importEdits returns the import changes a step's edits need: jetstream,
// context and nats added when used and missing, and imports left unused
// removed. The edits are in the coordinates of text, before the step.
func importEdits(text []byte, its []intent) ([]intent, error) {
	after := applyText(text, its)
	fset := token.NewFileSet()
	af, err := parser.ParseFile(fset, "", after, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("the step's edits do not parse: %w", err)
	}
	used := usedNames(af)
	bf, err := parser.ParseFile(fset, "", text, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	tf := fset.File(bf.Pos())
	off := func(p token.Pos) int { return tf.Offset(p) }
	importEnd := off(bf.Name.End())
	for _, d := range bf.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT {
			importEnd = off(gd.End())
		}
	}
	for _, it := range its {
		if it.start < importEnd {
			return nil, fmt.Errorf("an edit lies in the import block")
		}
	}
	have := make(map[string]bool)
	var remove []*ast.ImportSpec
	for _, spec := range bf.Imports {
		name := importName(spec)
		p, _ := strconv.Unquote(spec.Path.Value)
		have[p] = true
		if name == "_" || name == "." {
			continue
		}
		if !used[name] {
			remove = append(remove, spec)
		}
	}
	var add []string
	for _, p := range []string{"context", natsModule, jsPath} {
		if used[knownNames[p]] && !have[p] {
			add = append(add, p)
		}
	}
	if len(add) == 0 && len(remove) == 0 {
		return nil, nil
	}
	lineStart := func(o int) int {
		for o > 0 && text[o-1] != '\n' {
			o--
		}
		return o
	}
	lineEnd := func(o int) int {
		for o < len(text) && text[o] != '\n' {
			o++
		}
		if o < len(text) {
			o++
		}
		return o
	}
	var out []intent
	removed := func(spec *ast.ImportSpec) bool { return slices.Contains(remove, spec) }
	var block *ast.GenDecl
	for _, d := range bf.Decls {
		if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT && gd.Lparen.IsValid() {
			block = gd
			break
		}
	}
	// Removals.
	for _, d := range bf.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		left := 0
		for _, s := range gd.Specs {
			if !removed(s.(*ast.ImportSpec)) {
				left++
			}
		}
		if left == 0 && (gd != block || len(add) == 0) {
			out = append(out, intent{start: lineStart(off(gd.Pos())), end: lineEnd(off(gd.End()))})
			if gd == block {
				block = nil
			}
			continue
		}
		for _, s := range gd.Specs {
			if removed(s.(*ast.ImportSpec)) {
				out = append(out, intent{start: lineStart(off(s.Pos())), end: lineEnd(off(s.End()))})
			}
		}
	}
	if len(add) == 0 {
		return out, nil
	}
	if block == nil {
		// Turn a single-line import into a block, or start one.
		var single *ast.GenDecl
		for _, d := range bf.Decls {
			if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.IMPORT && len(gd.Specs) == 1 && !removed(gd.Specs[0].(*ast.ImportSpec)) {
				single = gd
				break
			}
		}
		var specs []importLine
		for _, p := range add {
			specs = append(specs, importLine{path: p, text: strconv.Quote(p)})
		}
		if single != nil {
			spec := single.Specs[0].(*ast.ImportSpec)
			p, _ := strconv.Unquote(spec.Path.Value)
			specs = append(specs, importLine{path: p, text: string(text[off(spec.Pos()):off(spec.End())])})
		}
		body := importBlockBody(specs)
		if single != nil {
			out = append(out, intent{start: off(single.Pos()), end: off(single.End()), text: "import (\n" + body + ")"})
			return out, nil
		}
		o := lineEnd(off(bf.Name.End()))
		out = append(out, intent{start: o, end: o, text: "\nimport (\n" + body + ")\n"})
		return out, nil
	}
	// Insert each import in sorted position within the group of its kind:
	// the first group of standard-library imports, or the group holding
	// nats.go, else the last group of other imports. A missing group is
	// started before the first group or after the last one.
	var groups [][]*ast.ImportSpec
	lastLine := 0
	for _, sp := range block.Specs {
		spec := sp.(*ast.ImportSpec)
		if removed(spec) {
			continue
		}
		first := spec.Pos()
		if spec.Doc != nil {
			first = spec.Doc.Pos()
		}
		if len(groups) == 0 || tf.Line(first) > lastLine+1 {
			groups = append(groups, nil)
		}
		groups[len(groups)-1] = append(groups[len(groups)-1], spec)
		lastLine = tf.Line(spec.End())
	}
	specPath := func(spec *ast.ImportSpec) string {
		p, _ := strconv.Unquote(spec.Path.Value)
		return p
	}
	specStart := func(spec *ast.ImportSpec) int {
		if spec.Doc != nil {
			return lineStart(off(spec.Doc.Pos()))
		}
		return lineStart(off(spec.Pos()))
	}
	type insertion struct {
		std, other []string
		newGroup   bool
	}
	at := make(map[int]*insertion)
	var offsets []int
	insert := func(o int, p string, newGroup bool) {
		ins := at[o]
		if ins == nil {
			ins = &insertion{newGroup: newGroup}
			at[o] = ins
			offsets = append(offsets, o)
		}
		if stdImport(p) {
			ins.std = append(ins.std, p)
		} else {
			ins.other = append(ins.other, p)
		}
	}
	for _, p := range add {
		target := -1
		for gi, g := range groups {
			if stdImport(p) && stdImport(specPath(g[0])) {
				target = gi
				break
			}
			if !stdImport(p) && !stdImport(specPath(g[0])) {
				target = gi
				if slices.ContainsFunc(g, func(s *ast.ImportSpec) bool { return specPath(s) == natsModule }) {
					break
				}
			}
		}
		switch {
		case target >= 0:
			g := groups[target]
			o := lineEnd(off(g[len(g)-1].End()))
			for _, spec := range g {
				if specPath(spec) > p {
					o = specStart(spec)
					break
				}
			}
			insert(o, p, false)
		case len(groups) == 0:
			insert(lineStart(off(block.Rparen)), p, false)
		case stdImport(p):
			insert(specStart(groups[0][0]), p, true)
		default:
			last := groups[len(groups)-1]
			insert(lineEnd(off(last[len(last)-1].End())), p, true)
		}
	}
	for _, o := range offsets {
		ins := at[o]
		var specs []importLine
		for _, p := range append(ins.std, ins.other...) {
			specs = append(specs, importLine{path: p, text: strconv.Quote(p)})
		}
		body := importBlockBody(specs)
		switch {
		case ins.newGroup && len(ins.std) > 0:
			body += "\n"
		case ins.newGroup:
			body = "\n" + body
		}
		out = append(out, intent{start: o, end: o, text: body})
	}
	return out, nil
}

// importLine is one import spec to write, with its path for sorting.
type importLine struct{ path, text string }

// importBlockBody returns the lines of an import block holding specs: the
// standard-library imports sorted, a blank line, then the others sorted.
func importBlockBody(specs []importLine) string {
	slices.SortFunc(specs, func(a, b importLine) int { return strings.Compare(a.path, b.path) })
	var std, other strings.Builder
	for _, s := range specs {
		b := &other
		if stdImport(s.path) {
			b = &std
		}
		b.WriteString("\t" + s.text + "\n")
	}
	if std.Len() > 0 && other.Len() > 0 {
		return std.String() + "\n" + other.String()
	}
	return std.String() + other.String()
}

// stdImport reports whether an import path is in the standard library:
// its first element has no dot.
func stdImport(p string) bool {
	return !strings.Contains(strings.Split(p, "/")[0], ".")
}

// usedNames returns the identifiers used as package qualifiers in f.
func usedNames(f *ast.File) map[string]bool {
	used := make(map[string]bool)
	ast.Inspect(f, func(n ast.Node) bool {
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if id, ok := sel.X.(*ast.Ident); ok {
				used[id.Name] = true
			}
		}
		return true
	})
	return used
}
