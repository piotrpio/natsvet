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

	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

// legacyUse is one identifier resolving to a legacy JetStream symbol.
type legacyUse struct {
	id  *ast.Ident
	sym string // mapping table key: "Type", "Func" or "Type.Method"
	// value marks a constant of a legacy type or a JetStream error value:
	// not a legacy symbol (legacyjs does not report it), but rewritten with
	// the values around it.
	value bool
}

// anchorKind says what kind of node a site is anchored at.
type anchorKind int

const (
	anchorCall anchorKind = iota + 1 // a call or conversion
	anchorLit                        // a composite literal of a legacy type
	anchorDecl                       // a field, parameter, result, variable or type declaration
	anchorRef                        // any other expression naming a legacy symbol
)

// site is the unit of rewriting: a legacy call with its options, config
// literals and (for subscribe calls) its handler, a declaration, or a
// reference.
type site struct {
	file   *srcFile
	anchor ast.Node
	kind   anchorKind
	stack  []ast.Node // ancestors of anchor, outermost first, anchor excluded
	sym    string     // the anchor's own legacy symbol
	uses   []legacyUse
	// dependents are sites on values this site's call produces; their uses
	// are part of uses.
	dependents []*site
	mergedInto *site
}

// inventory finds every legacy use in prog and groups the uses into sites,
// in file and position order.
func inventory(prog *program) []*site {
	var sites []*site
	for _, f := range prog.files {
		byAnchor := make(map[ast.Node]*site)
		var order []*site
		ins := inspector.New([]*ast.File{f.ast})
		ins.WithStack([]ast.Node{(*ast.Ident)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
			if !push {
				return true
			}
			id := n.(*ast.Ident)
			sym, ok := legacyKey(f.info().Uses[id])
			value := false
			if !ok {
				sym, ok = prog.valueRef(f.info().Uses[id])
				value = true
			}
			if !ok {
				return true
			}
			anchor, kind, depth := anchorOf(f.info(), stack)
			s := byAnchor[anchor]
			if s == nil {
				s = &site{file: f, anchor: anchor, kind: kind, stack: slices.Clone(stack[:depth])}
				s.sym = anchorSymbol(f.info(), anchor, kind)
				byAnchor[anchor] = s
				order = append(order, s)
			}
			s.uses = append(s.uses, legacyUse{id: id, sym: sym, value: value})
			return true
		})
		slices.SortFunc(order, func(a, b *site) int { return int(a.anchor.Pos() - b.anchor.Pos()) })
		sites = append(sites, mergeDependents(f, order)...)
	}
	return sites
}

// mergeDependents folds a legacy call made on a value that another site's
// call produced (e, err := kv.Get(k); e.Value()) into the producing site:
// the value's type changes only when its producer is rewritten, so the two
// are one unit. Values are followed through := definitions and range
// clauses within the file.
func mergeDependents(f *srcFile, sites []*site) []*site {
	info := f.info()
	byCall := make(map[ast.Node]*site)
	for _, s := range sites {
		if s.kind == anchorCall {
			byCall[s.anchor] = s
		}
	}
	// producer maps a defined variable to the expression it was produced
	// from: the call of a := definition, or the ranged expression.
	producer := make(map[types.Object]ast.Expr)
	ast.Inspect(f.ast, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			if n.Tok == token.DEFINE && len(n.Rhs) == 1 {
				for _, l := range n.Lhs {
					if id, ok := l.(*ast.Ident); ok && info.Defs[id] != nil {
						producer[info.Defs[id]] = n.Rhs[0]
					}
				}
			}
		case *ast.RangeStmt:
			if n.Tok == token.DEFINE {
				for _, e := range []ast.Expr{n.Key, n.Value} {
					if id, ok := e.(*ast.Ident); ok && info.Defs[id] != nil {
						producer[info.Defs[id]] = n.X
					}
				}
			}
		}
		return true
	})
	var owner func(e ast.Expr, depth int) *site
	owner = func(e ast.Expr, depth int) *site {
		if depth > 8 {
			return nil
		}
		switch x := ast.Unparen(e).(type) {
		case *ast.CallExpr:
			if s := byCall[x]; s != nil {
				return s
			}
			// A value derived by a call on another site's value (m from
			// sub.NextMsg) belongs to that site.
			if sel, ok := ast.Unparen(x.Fun).(*ast.SelectorExpr); ok {
				if _, handle := siblingType(info.TypeOf(sel.X)); !handle {
					return owner(sel.X, depth+1)
				}
			}
		case *ast.Ident:
			if src, ok := producer[info.Uses[x]]; ok {
				return owner(src, depth+1)
			}
		}
		return nil
	}
	merged := make(map[*site]bool)
	for _, s := range sites {
		call, ok := s.anchor.(*ast.CallExpr)
		if !ok {
			continue
		}
		sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
		if !ok {
			continue
		}
		if _, handle := siblingType(info.TypeOf(sel.X)); handle {
			continue
		}
		o := owner(sel.X, 0)
		for o != nil && merged[o] {
			o = o.mergedInto
		}
		if o == nil || o == s {
			continue
		}
		o.uses = append(o.uses, s.uses...)
		o.dependents = append(o.dependents, s)
		s.mergedInto = o
		merged[s] = true
	}
	var out []*site
	for _, s := range sites {
		if !merged[s] {
			out = append(out, s)
		}
	}
	return out
}

// valueRef reports whether obj is a package-level constant of package nats
// with a legacy type (an enum value), or an error value the jetstream
// package declares under the same name.
func (prog *program) valueRef(obj types.Object) (string, bool) {
	if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != natsModule || obj.Parent() != obj.Pkg().Scope() || !obj.Exported() {
		return "", false
	}
	switch obj.(type) {
	case *types.Const:
		return obj.Name(), hasLegacyType(obj.Type())
	case *types.Var:
		if !strings.HasPrefix(obj.Name(), "Err") || prog.js == nil {
			return "", false
		}
		return obj.Name(), prog.js.Scope().Lookup(obj.Name()) != nil
	}
	return "", false
}

// legacyKey returns the mapping table key of a legacy object.
func legacyKey(obj types.Object) (string, bool) {
	sym, ok := natsapi.LegacySymbol(obj)
	if !ok {
		return "", false
	}
	return strings.TrimPrefix(sym, "nats."), true
}

// anchorOf climbs from the identifier at the top of stack to the node its
// site is anchored at, and returns that node, its kind and its depth in
// stack (the index of the anchor).
func anchorOf(info *types.Info, stack []ast.Node) (ast.Node, anchorKind, int) {
	i := len(stack) - 1
	node := stack[i]
	kind := anchorRef
	up := func() ast.Node {
		if i == 0 {
			return nil
		}
		return stack[i-1]
	}
	if sel, ok := up().(*ast.SelectorExpr); ok && sel.Sel == node {
		node, i = sel, i-1
	}
	switch p := up().(type) {
	case *ast.CallExpr:
		if p.Fun == node {
			node, kind, i = p, anchorCall, i-1
		}
	default:
		// A type expression: climb to the literal or declaration using it.
		j := i
		for j > 0 && isTypeExpr(stack[j-1], stack[j]) {
			j--
		}
		if j == 0 {
			break
		}
		switch t := stack[j-1].(type) {
		case *ast.CompositeLit:
			if t.Type == stack[j] {
				node, kind, i = t, anchorLit, j-1
			}
		case *ast.Field, *ast.ValueSpec, *ast.TypeSpec:
			node, kind, i = t, anchorDecl, j-1
		}
	}
	if kind == anchorDecl {
		return node, kind, i
	}
	// Options, config literals and handlers belong to the legacy call whose
	// arguments contain them.
	for j := i - 1; j >= 0; j-- {
		switch a := stack[j].(type) {
		case ast.Stmt, *ast.FuncDecl:
			return node, kind, i
		case *ast.FuncLit:
			if !insideSubscribeArg(info, stack, j) {
				return node, kind, i
			}
		case *ast.CallExpr:
			if isLegacyCall(info, a) && inArgs(a, stack[j+1]) {
				node, kind, i = a, anchorCall, j
			}
		}
	}
	return node, kind, i
}

// isTypeExpr reports whether child is the type part of parent when parent
// is itself a type expression.
func isTypeExpr(parent, child ast.Node) bool {
	switch p := parent.(type) {
	case *ast.StarExpr:
		return p.X == child
	case *ast.ArrayType:
		return p.Elt == child
	case *ast.MapType:
		return p.Key == child || p.Value == child
	case *ast.ChanType:
		return p.Value == child
	case *ast.Ellipsis:
		return p.Elt == child
	case *ast.SelectorExpr:
		return p.Sel == child
	}
	return false
}

// insideSubscribeArg reports whether the function literal at stack[j] is an
// argument of a legacy subscribe call.
func insideSubscribeArg(info *types.Info, stack []ast.Node, j int) bool {
	if j == 0 {
		return false
	}
	call, ok := stack[j-1].(*ast.CallExpr)
	if !ok || !inArgs(call, stack[j]) {
		return false
	}
	sym, ok := calleeKey(info, call)
	return ok && table[sym].Kind == Subscribe
}

func inArgs(call *ast.CallExpr, n ast.Node) bool {
	for _, a := range call.Args {
		if a == n {
			return true
		}
	}
	return false
}

// calleeKey returns the legacy table key of a call's callee: a function, a
// method, or a legacy type used as a conversion.
func calleeKey(info *types.Info, call *ast.CallExpr) (string, bool) {
	if obj := typeutil.Callee(info, call); obj != nil {
		return legacyKey(obj)
	}
	var id *ast.Ident
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = fun
	case *ast.SelectorExpr:
		id = fun.Sel
	}
	if id == nil {
		return "", false
	}
	return legacyKey(info.Uses[id])
}

func isLegacyCall(info *types.Info, call *ast.CallExpr) bool {
	_, ok := calleeKey(info, call)
	return ok
}

// anchorSymbol returns the legacy symbol the anchor itself names.
func anchorSymbol(info *types.Info, anchor ast.Node, kind anchorKind) string {
	var id *ast.Ident
	switch kind {
	case anchorCall:
		sym, _ := calleeKey(info, anchor.(*ast.CallExpr))
		return sym
	case anchorLit:
		id = typeIdent(anchor.(*ast.CompositeLit).Type)
	case anchorDecl:
		var t ast.Expr
		switch d := anchor.(type) {
		case *ast.Field:
			t = d.Type
		case *ast.ValueSpec:
			t = d.Type
		case *ast.TypeSpec:
			t = d.Type
		}
		id = typeIdent(t)
	case anchorRef:
		switch r := anchor.(type) {
		case *ast.SelectorExpr:
			id = r.Sel
		case *ast.Ident:
			id = r
		}
	}
	if id == nil {
		return ""
	}
	sym, _ := legacyKey(info.Uses[id])
	return sym
}

// typeIdent returns the identifier naming the named type inside a type
// expression (through pointers, slices, maps, channels and qualifiers).
func typeIdent(t ast.Expr) *ast.Ident {
	for {
		switch e := t.(type) {
		case *ast.StarExpr:
			t = e.X
		case *ast.ArrayType:
			t = e.Elt
		case *ast.MapType:
			t = e.Value
		case *ast.ChanType:
			t = e.Value
		case *ast.Ellipsis:
			t = e.Elt
		case *ast.SelectorExpr:
			return e.Sel
		case *ast.Ident:
			return e
		default:
			return nil
		}
	}
}
