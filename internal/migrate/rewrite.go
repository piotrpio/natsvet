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
	"go/types"
	"regexp"
	"slices"
	"strings"
)

const jsPath = natsModule + "/jetstream"

// fieldRenames maps legacy struct fields whose jetstream counterpart has
// another name.
var fieldRenames = map[string]string{
	"ConsumerConfig.Heartbeat": "IdleHeartbeat",
}

var legacyTypeRef = regexp.MustCompile(`github\.com/nats-io/nats\.go\.([A-Za-z0-9_]+)`)

// typeKey renders a type with full package paths, so that types from the
// module's load and from the separately loaded jetstream package compare.
func typeKey(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string { return p.Path() })
}

// mappedTypeKey renders a legacy type as the jetstream type it becomes.
func mappedTypeKey(t types.Type) string {
	return legacyTypeRef.ReplaceAllStringFunc(typeKey(t), func(s string) string {
		name := s[len(natsModule)+1:]
		if e, ok := table[name]; ok && e.Kind == Rename {
			return jsPath + "." + e.Target
		}
		return s
	})
}

// hasLegacyType reports whether t mentions a legacy type.
func hasLegacyType(t types.Type) bool {
	return t != nil && mappedTypeKey(t) != typeKey(t)
}

// jsObject returns the object a jetstream target names: "Name" or
// "Type.Member".
func (prog *program) jsObject(target string) types.Object {
	if prog.js == nil {
		return nil
	}
	typ, member, ok := strings.Cut(target, ".")
	obj := prog.js.Scope().Lookup(typ)
	if !ok || obj == nil {
		return obj
	}
	m, _, _ := types.LookupFieldOrMethod(obj.Type(), true, prog.js, member)
	return m
}

// jsSignature returns the signature of a jetstream method or function.
func (prog *program) jsSignature(target string) *types.Signature {
	if f, ok := prog.jsObject(target).(*types.Func); ok {
		return f.Type().(*types.Signature)
	}
	return nil
}

// jsField reports the type of field name of the jetstream struct typ.
func (prog *program) jsField(typ, name string) (types.Type, bool) {
	obj := prog.jsObject(typ)
	if obj == nil {
		return nil, false
	}
	if _, ok := obj.Type().Underlying().(*types.Struct); !ok {
		return nil, false
	}
	f, _, _ := types.LookupFieldOrMethod(obj.Type(), true, prog.js, name)
	if v, ok := f.(*types.Var); ok && v.IsField() {
		return v.Type(), true
	}
	return nil, false
}

// resultFit says how a legacy call's results relate to its replacement's.
type resultFit int

const (
	// fitSame: identical types, usable in any context.
	fitSame resultFit = iota
	// fitRetyped: the jetstream counterparts of legacy types; new variables
	// defined from them change type.
	fitRetyped
	// fitDifferent: types that do not correspond; the results must be
	// discarded.
	fitDifferent
)

// fitResults compares a legacy signature's results with new results.
func fitResults(old *types.Tuple, newTypes []types.Type) []resultFit {
	out := make([]resultFit, old.Len())
	for i := range old.Len() {
		ot := old.At(i).Type()
		switch {
		case i >= len(newTypes) || mappedTypeKey(ot) != typeKey(newTypes[i]):
			out[i] = fitDifferent
		case hasLegacyType(ot):
			out[i] = fitRetyped
		default:
			out[i] = fitSame
		}
	}
	return out
}

func tupleTypes(t *types.Tuple) []types.Type {
	out := make([]types.Type, t.Len())
	for i := range t.Len() {
		out[i] = t.At(i).Type()
	}
	return out
}

// fresh returns base, or base with a number appended, that is not in
// avoid.
func fresh(base string, avoid map[string]bool) string {
	name := base
	for i := 2; avoid[name]; i++ {
		name = fmt.Sprintf("%s%d", base, i)
	}
	return name
}

// idents returns the names of every identifier in the nodes.
func idents(nodes ...ast.Node) map[string]bool {
	out := make(map[string]bool)
	for _, n := range nodes {
		if n == nil {
			continue
		}
		ast.Inspect(n, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				out[id.Name] = true
			}
			return true
		})
	}
	return out
}

// zeroResults returns the zero values to return with an error from a
// function whose results are types.
func zeroResults(ts []types.Type) []string {
	var out []string
	for _, t := range ts[:len(ts)-1] {
		switch u := t.Underlying().(type) {
		case *types.Pointer, *types.Interface, *types.Slice, *types.Map, *types.Chan, *types.Signature:
			out = append(out, "nil")
		case *types.Basic:
			switch {
			case u.Info()&types.IsString != 0:
				out = append(out, `""`)
			case u.Info()&types.IsBoolean != 0:
				out = append(out, "false")
			default:
				out = append(out, "0")
			}
		default:
			out = append(out, jsTypeText(t)+"{}")
		}
	}
	return out
}

// jsTypeText renders a type as source in a file importing nats and
// jetstream under their package names, with legacy types mapped.
func jsTypeText(t types.Type) string {
	s := types.TypeString(t, func(p *types.Package) string { return p.Name() })
	return legacyTypeText(s)
}

var legacyTypeName = regexp.MustCompile(`\bnats\.([A-Za-z0-9_]+)`)

// legacyTypeText maps "nats.X" in a rendered type to its jetstream name.
func legacyTypeText(s string) string {
	return legacyTypeName.ReplaceAllStringFunc(s, func(m string) string {
		if e, ok := table[m[len("nats."):]]; ok && e.Kind == Rename {
			return "jetstream." + e.Target
		}
		return m
	})
}

// problem is a reason a site cannot be rewritten mechanically.
type problem struct {
	class  string // classGuided or classUnmapped
	reason string
}

// rewriter builds the edits of one site in original coordinates.
type rewriter struct {
	prog     *program
	f        *srcFile
	problems []problem
	// notes are shown on the site when the rewrite is the one applied.
	notes []string
	// extra are edits outside the site's anchor (a handler function).
	extra []intent
	// templates are code shapes a guided site shows the agent.
	templates []string
}

func (rw *rewriter) guided(format string, args ...any) {
	rw.problems = append(rw.problems, problem{classGuided, fmt.Sprintf(format, args...)})
}

func (rw *rewriter) unmapped(reason string) {
	rw.problems = append(rw.problems, problem{classUnmapped, reason})
}

// refIntents renames the legacy references among uses that lie inside n:
// types, constants, error values and functions with a same-shaped
// jetstream counterpart. Methods of kind Same need nothing; any other
// legacy use inside n is a problem the caller did not handle.
func (rw *rewriter) refIntents(n ast.Node, uses []legacyUse, handled map[*ast.Ident]bool) []intent {
	var out []intent
	for _, u := range uses {
		if u.id.Pos() < n.Pos() || u.id.End() > n.End() || handled[u.id] {
			continue
		}
		if u.value {
			if it, ok := rw.valueIntent(u, ""); ok {
				out = append(out, it)
			}
			continue
		}
		e := table[u.sym]
		switch e.Kind {
		case Same:
			continue
		case Rename:
			if strings.Contains(e.Target, ".") {
				rw.guided("%s has no same-shaped counterpart", u.sym)
				continue
			}
			sel := rw.selectorOf(u.id)
			if sel == nil {
				rw.guided("%s is not referred to through its package name", u.sym)
				continue
			}
			out = append(out, rw.prog.replace(sel, "jetstream."+e.Target))
		case Unmapped:
			rw.unmapped(fmt.Sprintf("%s: %s", u.sym, e.Note))
		default:
			rw.guided("%s: %s", u.sym, entryHint(u.sym, e))
		}
	}
	return out
}

// valueIntent renames a constant or error value to its jetstream
// counterpart: target, or the same name.
func (rw *rewriter) valueIntent(u legacyUse, target string) (intent, bool) {
	if target == "" {
		target = u.sym
	}
	obj := rw.f.info().Uses[u.id]
	js := rw.prog.jsObject(target)
	if js == nil {
		rw.guided("nats.%s has no jetstream counterpart named %s", u.sym, target)
		return intent{}, false
	}
	if _, isConst := obj.(*types.Const); isConst && mappedTypeKey(obj.Type()) != typeKey(js.Type()) {
		rw.guided("nats.%s and jetstream.%s have different types", u.sym, target)
		return intent{}, false
	}
	sel := rw.selectorOf(u.id)
	if sel == nil {
		rw.guided("nats.%s is not referred to through its package name", u.sym)
		return intent{}, false
	}
	return rw.prog.replace(sel, "jetstream."+target), true
}

func entryHint(sym string, e Entry) string {
	if e.Note != "" {
		return e.Note
	}
	if e.Target != "" {
		return "becomes jetstream " + e.Target
	}
	return "has no mechanical rewrite in this position"
}

// selectorOf returns the qualified identifier `nats.X` whose Sel is id.
func (rw *rewriter) selectorOf(id *ast.Ident) *ast.SelectorExpr {
	var found *ast.SelectorExpr
	ast.Inspect(rw.f.ast, func(n ast.Node) bool {
		if found != nil || n == nil || n.Pos() > id.Pos() || n.End() < id.End() {
			return false
		}
		if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel == id {
			if x, ok := sel.X.(*ast.Ident); ok {
				if _, ok := rw.f.info().Uses[x].(*types.PkgName); ok {
					found = sel
				}
			}
			return false
		}
		return true
	})
	return found
}

// litIntents rewrites a composite literal of a legacy type in place: its
// type and every legacy reference inside it, keys whose field is renamed,
// and, for a consumer config without an ack policy, the explicit
// AckNonePolicy that legacy's zero value meant.
func (rw *rewriter) litIntents(lit *ast.CompositeLit, uses []legacyUse) []intent {
	out := rw.refIntents(lit, uses, nil)
	info := rw.f.info()
	var visit func(lit *ast.CompositeLit)
	visit = func(lit *ast.CompositeLit) {
		t := info.TypeOf(lit)
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		named, _ := types.Unalias(t).(*types.Named)
		legacy := false
		if named != nil {
			var sym string
			sym, legacy = legacyKey(named.Obj())
			if legacy && table[sym].Kind != Rename {
				rw.guided("a nats.%s literal: %s", sym, entryHint(sym, table[sym]))
				legacy = false
			}
		}
		_, isStruct := t.Underlying().(*types.Struct)
		hasAck := false
		for _, elt := range lit.Elts {
			v := elt
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				v = kv.Value
				if key, ok := kv.Key.(*ast.Ident); ok && isStruct && legacy {
					name := named.Obj().Name()
					if key.Name == "AckPolicy" {
						hasAck = true
					}
					if nn, ok := fieldRenames[name+"."+key.Name]; ok {
						out = append(out, rw.prog.replace(key, nn))
					} else if _, ok := rw.prog.jsField(table[name].Target, key.Name); !ok {
						rw.guided("field %s.%s has no jetstream counterpart", name, key.Name)
					}
				}
			}
			if inner, ok := ast.Unparen(v).(*ast.CompositeLit); ok {
				visit(inner)
			} else if u, ok := ast.Unparen(v).(*ast.UnaryExpr); ok {
				if inner, ok := ast.Unparen(u.X).(*ast.CompositeLit); ok {
					visit(inner)
				}
			}
		}
		if legacy && named.Obj().Name() == "ConsumerConfig" && !hasAck {
			out = append(out, rw.ackNone(lit))
		}
	}
	visit(lit)
	return out
}

// ackNone adds AckPolicy: jetstream.AckNonePolicy to a consumer config
// literal: legacy's zero AckPolicy is AckNonePolicy, jetstream's is
// AckExplicitPolicy.
func (rw *rewriter) ackNone(lit *ast.CompositeLit) intent {
	const field = "AckPolicy: jetstream.AckNonePolicy"
	prog := rw.prog
	if len(lit.Elts) == 0 {
		return prog.insertAt(lit.Rbrace, field)
	}
	last := lit.Elts[len(lit.Elts)-1]
	lp, rp := prog.fset.Position(last.End()), prog.fset.Position(lit.Rbrace)
	if lp.Line == rp.Line {
		return prog.insertAt(last.End(), ", "+field)
	}
	// One element per line: add a line after the last element, which ends
	// with a comma.
	_, end := prog.lineSpan(last.Pos(), last.End())
	return intent{file: rw.f, start: end, end: end, text: prog.indentOf(last.Pos()) + field + ",\n"}
}

// sortIntents orders intents by start offset and reports whether any two
// overlap.
func sortIntents(in []intent) ([]intent, bool) {
	slices.SortFunc(in, func(a, b intent) int {
		if a.file.rel != b.file.rel {
			return strings.Compare(a.file.rel, b.file.rel)
		}
		if a.start != b.start {
			return a.start - b.start
		}
		return a.end - b.end
	})
	for i := 1; i < len(in); i++ {
		a, b := in[i-1], in[i]
		if a.file == b.file && (b.start < a.end || (a.start == b.start && a.end == b.end && a.start == a.end && a.text != b.text)) {
			return in, true
		}
	}
	return in, false
}
