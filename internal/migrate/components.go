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
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

// handleTypes maps a legacy handle type to the type of its jetstream
// sibling.
var handleTypes = map[string]string{
	"JetStreamContext":   "jetstream.JetStream",
	"JetStream":          "jetstream.JetStream",
	"JetStreamManager":   "jetstream.JetStream",
	"KeyValueManager":    "jetstream.JetStream",
	"ObjectStoreManager": "jetstream.JetStream",
	"KeyValue":           "jetstream.KeyValue",
	"ObjectStore":        "jetstream.ObjectStore",
}

// siblingType returns the jetstream type of a legacy handle type.
func siblingType(t types.Type) (string, bool) {
	n, ok := types.Unalias(t).(*types.Named)
	if !ok || !natsapi.IsPkg(n.Obj(), natsapi.Core) {
		return "", false
	}
	st, ok := handleTypes[n.Obj().Name()]
	return st, ok
}

// handleKind says where a handle-carrying object is declared.
type handleKind int

const (
	handleLocal handleKind = iota + 1
	handleParam
	handleResult
	handleField
)

// handle is a variable, parameter, result or struct field whose type is a
// legacy handle.
type handle struct {
	key     objKey
	kind    handleKind
	file    *srcFile
	ident   *ast.Ident // declaring identifier
	newType string
	sibling string // name of the jetstream sibling
	// threadable: a sibling can be declared and fed next to this handle.
	threadable bool
	why        string
	// present is the declaring identifier of the sibling when the loaded
	// code already has it, as an earlier add-handle step left it.
	present *ast.Ident
}

// flowKind says how a handle value reaches another handle.
type flowKind int

const (
	flowAssign   flowKind = iota + 1 // a = b or a := b
	flowLitField                     // T{Field: b}
	flowArg                          // f(b) into a parameter
	flowReturn                       // return b into a result
)

type flow struct {
	kind     flowKind
	file     *srcFile
	node     ast.Node // statement, key-value expression, call or return
	from, to objKey
	fromExpr ast.Expr
	toExpr   ast.Expr // left-hand side for assignments; nil otherwise
	argIndex int
	define   bool
}

// root is a call that creates a legacy handle: nc.JetStream, or a
// KeyValue or object store handle taken from a JetStream handle.
type root struct {
	file     *srcFile
	stmt     *ast.AssignStmt
	call     *ast.CallExpr
	sym      string
	to       objKey
	recv     objKey // the parent handle of a derived root
	errCheck *ast.IfStmt
	site     *site
	why      string // why the root cannot be recreated mechanically
}

// boundary is a handle leaving the loaded packages or used in a way the
// planner does not thread.
type boundary struct {
	file   *srcFile
	pos    token.Pos
	obj    objKey
	reason string
}

// component is a set of handles connected by flows and derivation.
type component struct {
	id         string
	handles    []*handle
	roots      []*root
	flows      []*flow
	boundaries []boundary
	sites      []*site
	skipped    bool
	oneCommit  bool
}

// graph holds every handle, flow and root of the program.
type graph struct {
	prog    *program
	handles map[objKey]*handle
	flows   []*flow
	roots   []*root
	bounds  []boundary
	parent  map[objKey]objKey
}

func (g *graph) find(k objKey) objKey {
	for g.parent[k] != k {
		g.parent[k] = g.parent[g.parent[k]]
		k = g.parent[k]
	}
	return k
}

func (g *graph) union(a, b objKey) {
	if _, ok := g.parent[a]; !ok {
		g.parent[a] = a
	}
	if _, ok := g.parent[b]; !ok {
		g.parent[b] = b
	}
	ra, rb := g.find(a), g.find(b)
	if ra != rb {
		g.parent[rb] = ra
	}
}

// handleOf returns the handle an expression denotes: an identifier or a
// selector naming a variable or field of a legacy handle type.
func (g *graph) handleOf(info *types.Info, e ast.Expr) (*handle, bool) {
	var id *ast.Ident
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		id = x
	case *ast.SelectorExpr:
		id = x.Sel
	default:
		return nil, false
	}
	obj := info.ObjectOf(id)
	v, ok := obj.(*types.Var)
	if !ok {
		return nil, false
	}
	h := g.handles[g.prog.key(v)]
	return h, h != nil
}

// buildGraph finds every handle, flow, root and boundary in prog.
func buildGraph(prog *program, sites []*site) *graph {
	g := &graph{prog: prog, handles: make(map[objKey]*handle), parent: make(map[objKey]objKey)}
	g.collectHandles()
	rootSites := make(map[*ast.CallExpr]*site)
	for _, s := range sites {
		if call, ok := s.anchor.(*ast.CallExpr); ok {
			rootSites[call] = s
		}
	}
	for _, f := range prog.files {
		g.collectFlows(f, rootSites)
	}
	g.collectOtherUses()
	for k := range g.handles {
		if _, ok := g.parent[k]; !ok {
			g.parent[k] = k
		}
	}
	for _, fl := range g.flows {
		if fl.from != "" && fl.to != "" {
			g.union(fl.from, fl.to)
		}
	}
	for _, r := range g.roots {
		if r.recv != "" {
			g.union(r.recv, r.to)
		}
	}
	g.threadability()
	g.recognizeSiblings()
	return g
}

// recognizeSiblings finds the siblings an earlier add-handle step declared:
// a declaration named as siblingName would have named it before it
// existed (<name>New, or with a number), of the sibling's type, later in
// the same scope for a local, in the same struct for a field, or in the
// same parameter list for a parameter.
func (g *graph) recognizeSiblings() {
	for _, h := range g.handles {
		if !h.threadable {
			continue
		}
		base := h.ident.Name + "New"
		for i := 1; i < 10 && h.present == nil; i++ {
			name := base
			if i > 1 {
				name = fmt.Sprintf("%s%d", base, i)
			}
			if id := g.siblingCandidate(h, name); id != nil {
				h.present, h.sibling = id, name
			}
		}
	}
}

// siblingCandidate returns the declaration named name that sits where a
// sibling of h would, with the sibling's type.
func (g *graph) siblingCandidate(h *handle, name string) *ast.Ident {
	info := h.file.info()
	var cand *ast.Ident
	switch h.kind {
	case handleField, handleParam:
		path, _ := astutil.PathEnclosingInterval(h.file.ast, h.ident.Pos(), h.ident.End())
		for _, n := range path {
			fl, ok := n.(*ast.FieldList)
			if !ok {
				continue
			}
			for _, fld := range fl.List {
				for _, nm := range fld.Names {
					if nm.Name == name {
						cand = nm
					}
				}
			}
			break
		}
	case handleLocal:
		v := info.Defs[h.ident].(*types.Var)
		if v.Parent() == nil {
			return nil
		}
		o := v.Parent().Lookup(name)
		if o == nil || (v.Parent() != v.Pkg().Scope() && o.Pos() < h.ident.Pos()) {
			return nil
		}
		if f := g.prog.fileOf(o.Pos()); f != nil {
			cand = identAt(f.ast, o.Pos())
		}
	}
	if cand == nil {
		return nil
	}
	obj := g.prog.fileOf(cand.Pos()).info().Defs[cand]
	if obj == nil || types.TypeString(obj.Type(), func(p *types.Package) string { return p.Name() }) != h.newType {
		return nil
	}
	return cand
}

// identAt returns the identifier of f that starts at pos.
func identAt(f *ast.File, pos token.Pos) *ast.Ident {
	var out *ast.Ident
	ast.Inspect(f, func(n ast.Node) bool {
		if out != nil || n == nil || n.Pos() > pos || n.End() <= pos {
			return false
		}
		if id, ok := n.(*ast.Ident); ok && id.Pos() == pos {
			out = id
		}
		return true
	})
	return out
}

// isPlaceholder reports whether stmt is `_ = x`, the line an add-handle
// step writes to keep a local handle or sibling used.
func isPlaceholder(stmt ast.Node) (ast.Expr, bool) {
	as, ok := stmt.(*ast.AssignStmt)
	if !ok || as.Tok != token.ASSIGN || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
		return nil, false
	}
	if id, ok := as.Lhs[0].(*ast.Ident); !ok || id.Name != "_" {
		return nil, false
	}
	if _, ok := ast.Unparen(as.Rhs[0]).(*ast.Ident); !ok {
		return nil, false
	}
	return as.Rhs[0], true
}

// collectHandles records every declaration of a handle-typed object in the
// loaded files.
func (g *graph) collectHandles() {
	for _, f := range g.prog.files {
		info := f.info()
		for id, obj := range info.Defs {
			v, ok := obj.(*types.Var)
			if !ok || id.Name == "_" {
				continue
			}
			// Defs covers the whole package variant, other files and, in
			// a test variant, their own parse too.
			if id.Pos() < f.ast.FileStart || id.Pos() > f.ast.FileEnd {
				continue
			}
			nt, ok := siblingType(v.Type())
			if !ok {
				continue
			}
			h := &handle{key: g.prog.key(v), file: f, ident: id, newType: nt, threadable: true}
			path, _ := astutil.PathEnclosingInterval(f.ast, id.Pos(), id.End())
			h.kind = declKind(v, path)
			if h.kind == handleResult {
				h.threadable, h.why = false, "a function result carries the handle"
			}
			g.handles[h.key] = h
		}
	}
	for _, h := range g.handles {
		h.sibling = g.siblingName(h)
	}
}

func declKind(v *types.Var, path []ast.Node) handleKind {
	if v.IsField() {
		return handleField
	}
	for i, n := range path {
		ft, ok := n.(*ast.FuncType)
		if !ok || i == 0 {
			continue
		}
		if fl, ok := path[i-1].(*ast.FieldList); ok {
			if fl == ft.Results {
				return handleResult
			}
			return handleParam
		}
	}
	return handleLocal
}

// siblingName returns the name of a handle's sibling: the handle's name
// with "New" appended, or with a number added when that name is taken in
// the handle's scope or struct.
func (g *graph) siblingName(h *handle) string {
	v := h.file.info().Defs[h.ident].(*types.Var)
	taken := func(name string) bool {
		if h.kind == handleField {
			path, _ := astutil.PathEnclosingInterval(h.file.ast, h.ident.Pos(), h.ident.End())
			for _, n := range path {
				if st, ok := n.(*ast.StructType); ok {
					for _, fld := range st.Fields.List {
						for _, nm := range fld.Names {
							if nm.Name == name {
								return true
							}
						}
					}
					return false
				}
			}
			return false
		}
		if v.Parent() == nil {
			return false
		}
		_, obj := v.Parent().LookupParent(name, h.ident.Pos())
		if obj != nil {
			return true
		}
		for _, child := range scopesUnder(v.Parent()) {
			if child.Lookup(name) != nil {
				return true
			}
		}
		return false
	}
	base := h.ident.Name + "New"
	name := base
	for i := 2; taken(name); i++ {
		name = fmt.Sprintf("%s%d", base, i)
	}
	return name
}

func scopesUnder(s *types.Scope) []*types.Scope {
	var out []*types.Scope
	for i := range s.NumChildren() {
		c := s.Child(i)
		out = append(out, c)
		out = append(out, scopesUnder(c)...)
	}
	return out
}

// collectFlows records the flows, roots and boundaries of one file.
func (g *graph) collectFlows(f *srcFile, rootSites map[*ast.CallExpr]*site) {
	info := f.info()
	ins := inspector.New([]*ast.File{f.ast})
	ins.WithStack([]ast.Node{(*ast.AssignStmt)(nil), (*ast.ValueSpec)(nil), (*ast.CompositeLit)(nil), (*ast.CallExpr)(nil), (*ast.ReturnStmt)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return true
		}
		switch n := n.(type) {
		case *ast.AssignStmt:
			g.assignFlows(f, n, stack, rootSites)
		case *ast.ValueSpec:
			for i, name := range n.Names {
				if i >= len(n.Values) {
					break
				}
				to, ok := g.handles[g.prog.key(info.Defs[name])]
				if !ok {
					continue
				}
				if from, ok := g.handleOf(info, n.Values[i]); ok {
					g.flows = append(g.flows, &flow{kind: flowAssign, file: f, node: n, from: from.key, to: to.key, fromExpr: n.Values[i], toExpr: name, define: true})
				} else {
					to.threadable, to.why = false, "initialized in a var declaration the planner does not rewrite"
				}
			}
		case *ast.CompositeLit:
			for _, elt := range n.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := kv.Key.(*ast.Ident)
				if !ok {
					continue
				}
				to, ok := g.handles[g.prog.key(info.Uses[key])]
				if !ok {
					continue
				}
				if from, ok := g.handleOf(info, kv.Value); ok {
					g.flows = append(g.flows, &flow{kind: flowLitField, file: f, node: kv, from: from.key, to: to.key, fromExpr: kv.Value})
				} else {
					to.threadable, to.why = false, "set in a composite literal from an expression the planner does not rewrite"
				}
			}
		case *ast.CallExpr:
			g.argFlows(f, n)
		case *ast.ReturnStmt:
			for _, r := range n.Results {
				if from, ok := g.handleOf(info, r); ok {
					from.threadable, from.why = false, "returned from a function"
					g.flows = append(g.flows, &flow{kind: flowReturn, file: f, node: n, from: from.key, fromExpr: r})
				}
			}
		}
		return true
	})
}

// assignFlows handles an assignment: a root call, or handle-to-handle
// copies.
func (g *graph) assignFlows(f *srcFile, n *ast.AssignStmt, stack []ast.Node, rootSites map[*ast.CallExpr]*site) {
	info := f.info()
	if len(n.Rhs) == 1 && len(n.Lhs) >= 1 {
		if call, ok := ast.Unparen(n.Rhs[0]).(*ast.CallExpr); ok {
			if sym, ok := calleeKey(info, call); ok && isRootSym(sym) {
				to, ok := g.handleOf(info, n.Lhs[0])
				if !ok {
					return
				}
				r := &root{file: f, stmt: n, call: call, sym: sym, to: to.key, site: rootSites[call]}
				if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok && sym != "Conn.JetStream" {
					if parent, ok := g.handleOf(info, sel.X); ok {
						r.recv = parent.key
					} else {
						r.why = "the handle is derived from an expression the planner does not rewrite"
					}
				}
				if len(n.Lhs) != 2 {
					r.why = "the root does not assign the handle and an error"
				}
				r.errCheck = followingErrCheck(stack, n)
				g.roots = append(g.roots, r)
				return
			}
		}
	}
	if len(n.Lhs) != len(n.Rhs) {
		for _, l := range n.Lhs {
			if h, ok := g.handleOf(info, l); ok {
				h.threadable, h.why = false, "assigned from a multi-value call the planner does not rewrite"
			}
		}
		return
	}
	for i, l := range n.Lhs {
		to, ok := g.handleOf(info, l)
		if !ok {
			continue
		}
		from, ok := g.handleOf(info, n.Rhs[i])
		if !ok {
			to.threadable, to.why = false, "assigned from an expression the planner does not rewrite"
			continue
		}
		fl := &flow{kind: flowAssign, file: f, node: n, from: from.key, to: to.key, fromExpr: n.Rhs[i], toExpr: l, define: n.Tok == token.DEFINE}
		if len(n.Lhs) != 1 {
			to.threadable, to.why = false, "assigned in a multi-assignment"
		}
		g.flows = append(g.flows, fl)
	}
}

// isRootSym reports whether a legacy call creates a handle.
func isRootSym(sym string) bool {
	switch sym {
	case "Conn.JetStream", "KeyValueManager.KeyValue", "KeyValueManager.CreateKeyValue", "ObjectStoreManager.ObjectStore", "ObjectStoreManager.CreateObjectStore":
		return true
	}
	return false
}

// followingErrCheck returns the `if err != nil { ... }` statement that
// directly follows stmt in its block, if any.
func followingErrCheck(stack []ast.Node, stmt ast.Stmt) *ast.IfStmt {
	if len(stack) < 2 {
		return nil
	}
	block, ok := stack[len(stack)-2].(*ast.BlockStmt)
	if !ok {
		return nil
	}
	i := slices.Index(block.List, stmt)
	if i < 0 || i+1 >= len(block.List) {
		return nil
	}
	ifs, ok := block.List[i+1].(*ast.IfStmt)
	if !ok || ifs.Init != nil || ifs.Else != nil {
		return nil
	}
	bin, ok := ifs.Cond.(*ast.BinaryExpr)
	if !ok || bin.Op != token.NEQ {
		return nil
	}
	if id, ok := bin.X.(*ast.Ident); !ok || id.Name != "err" {
		return nil
	}
	if id, ok := bin.Y.(*ast.Ident); !ok || id.Name != "nil" {
		return nil
	}
	return ifs
}

// argFlows records handles passed to functions: a flow into a parameter
// of a loaded function, a boundary otherwise.
func (g *graph) argFlows(f *srcFile, call *ast.CallExpr) {
	info := f.info()
	fn, _ := typeutil.Callee(info, call).(*types.Func)
	if fn != nil {
		if _, legacy := legacyKey(fn); legacy {
			return
		}
	}
	for i, a := range call.Args {
		from, ok := g.handleOf(info, a)
		if !ok {
			// A handle parameter fed something other than a handle
			// variable (a mock, nil, a call) cannot get a sibling.
			if fn != nil {
				sig := fn.Type().(*types.Signature)
				if !sig.Variadic() || i < sig.Params().Len()-1 {
					if i < sig.Params().Len() {
						if to, ok := g.handles[g.prog.key(sig.Params().At(i))]; ok {
							to.threadable, to.why = false, "a caller passes it a value that is not a handle variable ("+types.ExprString(a)+")"
						}
					}
				}
			}
			continue
		}
		if fn == nil {
			from.threadable, from.why = false, "passed to a function value"
			g.bounds = append(g.bounds, boundary{file: f, pos: a.Pos(), obj: from.key, reason: "passed to a function value the planner cannot follow"})
			continue
		}
		sig := fn.Type().(*types.Signature)
		if sig.Variadic() && i >= sig.Params().Len()-1 {
			g.bounds = append(g.bounds, boundary{file: f, pos: a.Pos(), obj: from.key, reason: "passed as a variadic argument"})
			continue
		}
		param := sig.Params().At(i)
		to, ok := g.handles[g.prog.key(param)]
		if !ok || !g.prog.inLoaded(param) {
			g.bounds = append(g.bounds, boundary{file: f, pos: a.Pos(), obj: from.key, reason: fmt.Sprintf("passed to %s, outside the loaded packages", fn.FullName())})
			continue
		}
		g.flows = append(g.flows, &flow{kind: flowArg, file: f, node: call, from: from.key, to: to.key, fromExpr: a, argIndex: i})
	}
}

// collectOtherUses records every use of a handle that is not a legacy
// method receiver, a flow endpoint or a root: such a use would break when
// the legacy handle is removed.
func (g *graph) collectOtherUses() {
	known := make(map[token.Pos]bool)
	for _, fl := range g.flows {
		known[fl.fromExpr.Pos()] = true
		if fl.toExpr != nil {
			known[fl.toExpr.Pos()] = true
		}
	}
	for _, b := range g.bounds {
		known[b.pos] = true
	}
	for _, r := range g.roots {
		known[r.stmt.Lhs[0].Pos()] = true
		if sel, ok := ast.Unparen(r.call.Fun).(*ast.SelectorExpr); ok {
			known[sel.X.Pos()] = true
		}
	}
	for _, f := range g.prog.files {
		info := f.info()
		ins := inspector.New([]*ast.File{f.ast})
		ins.WithStack([]ast.Node{(*ast.Ident)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
			if !push {
				return true
			}
			id := n.(*ast.Ident)
			v, ok := info.Uses[id].(*types.Var)
			if !ok {
				return true
			}
			h, ok := g.handles[g.prog.key(v)]
			if !ok {
				return true
			}
			// The expression denoting the handle is the identifier, or the
			// selector ending in it; parent is the node above that.
			var expr ast.Node = id
			parent := len(stack) - 2
			if parent >= 0 {
				if sel, ok := stack[parent].(*ast.SelectorExpr); ok && sel.Sel == id {
					expr, parent = sel, parent-1
				}
			}
			if known[expr.Pos()] || parent < 0 {
				return true
			}
			if _, ok := isPlaceholder(stack[parent]); ok {
				return true
			}
			switch p := stack[parent].(type) {
			case *ast.SelectorExpr:
				if p.X == expr && isLegacyMethod(info, p) {
					return true
				}
			case *ast.KeyValueExpr:
				if p.Key == expr {
					return true
				}
			}
			g.bounds = append(g.bounds, boundary{file: f, pos: id.Pos(), obj: h.key, reason: "used in a way the planner does not thread"})
			return true
		})
	}
}

func isLegacyMethod(info *types.Info, sel *ast.SelectorExpr) bool {
	_, ok := legacyKey(info.Uses[sel.Sel])
	return ok
}

// threadability decides, as a greatest fixpoint, which handles can get a
// sibling: a handle is threadable when its own declaration allows it and
// every value flowing into it comes from a threadable handle or a root
// that can be recreated.
func (g *graph) threadability() {
	incoming := make(map[objKey][]objKey)
	for _, fl := range g.flows {
		if fl.to != "" {
			incoming[fl.to] = append(incoming[fl.to], fl.from)
		}
	}
	for _, r := range g.roots {
		if r.why != "" {
			if h := g.handles[r.to]; h != nil {
				h.threadable, h.why = false, r.why
			}
		}
		if r.recv != "" {
			incoming[r.to] = append(incoming[r.to], r.recv)
		}
	}
	for changed := true; changed; {
		changed = false
		for k, h := range g.handles {
			if !h.threadable {
				continue
			}
			for _, from := range incoming[k] {
				if src := g.handles[from]; src == nil || !src.threadable {
					h.threadable, h.why = false, "fed from a handle that cannot be threaded"
					changed = true
					break
				}
			}
		}
	}
}

// components groups the handles of g, with their roots, flows, boundaries
// and sites, into components in position order.
func (g *graph) components(sites []*site) []*component {
	byRoot := make(map[objKey]*component)
	get := func(k objKey) *component {
		r := g.find(k)
		c := byRoot[r]
		if c == nil {
			c = &component{}
			byRoot[r] = c
		}
		return c
	}
	for _, h := range g.handles {
		c := get(h.key)
		c.handles = append(c.handles, h)
	}
	for _, r := range g.roots {
		c := get(r.to)
		c.roots = append(c.roots, r)
	}
	for _, fl := range g.flows {
		if fl.to != "" {
			c := get(fl.to)
			c.flows = append(c.flows, fl)
		} else {
			c := get(fl.from)
			c.flows = append(c.flows, fl)
		}
	}
	for _, b := range g.bounds {
		c := get(b.obj)
		c.boundaries = append(c.boundaries, b)
	}
	for _, s := range sites {
		if k, ok := g.siteHandle(s); ok {
			c := get(k)
			c.sites = append(c.sites, s)
		}
	}
	var out []*component
	for _, c := range byRoot {
		if len(c.roots) == 0 && len(c.sites) == 0 {
			continue
		}
		slices.SortFunc(c.handles, func(a, b *handle) int { return g.cmpPos(a.ident.Pos(), b.ident.Pos()) })
		slices.SortFunc(c.roots, func(a, b *root) int { return g.cmpPos(a.stmt.Pos(), b.stmt.Pos()) })
		slices.SortFunc(c.sites, func(a, b *site) int { return g.cmpPos(a.anchor.Pos(), b.anchor.Pos()) })
		out = append(out, c)
	}
	slices.SortFunc(out, func(a, b *component) int { return g.cmpPos(firstPos(a), firstPos(b)) })
	seen := make(map[string]int)
	for _, c := range out {
		id := g.componentID(c)
		seen[id]++
		if n := seen[id]; n > 1 {
			id = fmt.Sprintf("%s.%d", id, n)
		}
		c.id = id
	}
	return out
}

// componentID names a component by the declaration of its first handle:
// <pkg>.<Type>.<field> for a struct field, else the enclosing function
// (its package at package level) and the handle's name, <pkg>.<Func>#<name>.
func (g *graph) componentID(c *component) string {
	if len(c.handles) == 0 {
		p := g.prog.fset.Position(firstPos(c))
		return fmt.Sprintf("%s:%d:%d", g.prog.fileOf(firstPos(c)).rel, p.Line, p.Column)
	}
	h := c.handles[0]
	path, _ := astutil.PathEnclosingInterval(h.file.ast, h.ident.Pos(), h.ident.End())
	scope, inFunc := g.prog.scope(h.file, path)
	if h.kind == handleField && !inFunc {
		for _, n := range path {
			if ts, ok := n.(*ast.TypeSpec); ok {
				return scope + "." + ts.Name.Name + "." + h.ident.Name
			}
		}
	}
	return scope + "#" + h.ident.Name
}

func firstPos(c *component) token.Pos {
	if len(c.roots) > 0 {
		return c.roots[0].stmt.Pos()
	}
	if len(c.handles) > 0 {
		return c.handles[0].ident.Pos()
	}
	return c.sites[0].anchor.Pos()
}

// cmpPos orders positions by file path, then offset.
func (g *graph) cmpPos(a, b token.Pos) int {
	pa, pb := g.prog.fset.Position(a), g.prog.fset.Position(b)
	if c := strings.Compare(pa.Filename, pb.Filename); c != 0 {
		return c
	}
	return pa.Offset - pb.Offset
}

// siteHandle returns the handle a site acts on: the receiver of its legacy
// call, or the handle a root or declaration creates.
func (g *graph) siteHandle(s *site) (objKey, bool) {
	info := s.file.info()
	switch a := s.anchor.(type) {
	case *ast.CallExpr:
		for _, r := range g.roots {
			if r.call == a {
				return r.to, true
			}
		}
		if sel, ok := ast.Unparen(a.Fun).(*ast.SelectorExpr); ok {
			if h, ok := g.handleOf(info, sel.X); ok {
				return h.key, true
			}
		}
	case *ast.Field:
		for _, n := range a.Names {
			if h, ok := g.handles[g.prog.key(info.Defs[n])]; ok {
				return h.key, true
			}
		}
	case *ast.ValueSpec:
		for _, n := range a.Names {
			if h, ok := g.handles[g.prog.key(info.Defs[n])]; ok {
				return h.key, true
			}
		}
	}
	return "", false
}
