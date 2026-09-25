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
		var std, other []string
		for _, p := range add {
			if p == "context" {
				std = append(std, p)
			} else {
				other = append(other, p)
			}
		}
		var sb strings.Builder
		sb.WriteString("import (\n")
		for _, p := range std {
			sb.WriteString("\t" + strconv.Quote(p) + "\n")
		}
		if single != nil {
			spec := single.Specs[0].(*ast.ImportSpec)
			if len(std) > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString("\t" + string(text[off(spec.Pos()):off(spec.End())]) + "\n")
			if len(other) > 0 {
				sb.WriteString("\n")
			}
		} else if len(std) > 0 && len(other) > 0 {
			sb.WriteString("\n")
		}
		for _, p := range other {
			sb.WriteString("\t" + strconv.Quote(p) + "\n")
		}
		sb.WriteString(")")
		if single != nil {
			out = append(out, intent{start: off(single.Pos()), end: off(single.End()), text: sb.String()})
			return out, nil
		}
		o := lineEnd(off(bf.Name.End()))
		out = append(out, intent{start: o, end: o, text: "\n" + sb.String() + "\n"})
		return out, nil
	}
	first := block.Specs[0].(*ast.ImportSpec)
	firstPath, _ := strconv.Unquote(first.Path.Value)
	for _, p := range add {
		line := "\t" + strconv.Quote(p) + "\n"
		switch p {
		case "context":
			o := lineStart(off(first.Pos()))
			if strings.Contains(strings.Split(firstPath, "/")[0], ".") {
				// The block starts with third-party imports: a group of
				// its own.
				line += "\n"
			}
			out = append(out, intent{start: o, end: o, text: line})
		default:
			o := lineStart(off(block.Rparen))
			for _, s := range block.Specs {
				sp := s.(*ast.ImportSpec)
				if v, _ := strconv.Unquote(sp.Path.Value); v == natsModule && p == jsPath {
					o = lineEnd(off(sp.End()))
				}
			}
			out = append(out, intent{start: o, end: o, text: line})
		}
	}
	return out, nil
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
