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
	"golang.org/x/tools/go/types/typeutil"
)

// siteRole says which steps migrate a site.
type siteRole int

const (
	// roleSite: the site's own step.
	roleSite siteRole = iota
	// roleRoot: a handle root; the add-handle step creates its sibling and
	// the removal step deletes it.
	roleRoot
	// roleDecl: a handle declaration; the add-handle step declares its
	// sibling and the removal step deletes it.
	roleDecl
)

// classified is a site with its class and rewrite.
type classified struct {
	s       *site
	id      string
	class   string
	summary string
	intents []intent
	// after overrides the after text shown for the site (roots).
	after     string
	template  string
	facts     []string
	notes     []string
	decisions []*decision
	followUps []FollowUp
	ref       string
	role      siteRole
	// owner is the site whose rewrite covers this one (a message method in
	// a handler another site rewrites).
	owner *classified
	// links are legacy-typed variables whose type the site's rewrite changes
	// or relies on.
	links []objKey
	unit  *unit
	// comp is the component whose handles the site acts on.
	comp    *component
	skipped bool
	// drop removes the site from the plan: an error value no migrated
	// call produces.
	drop bool
	// root is the sibling root statement of a mechanical roleRoot site.
	root *rootPlan
}

// rootPlan is how the add-handle step creates a root's sibling.
type rootPlan struct {
	r *root
	// text is the statement (or statements) inserted after the root and
	// its error check, with marks on sibling identifiers.
	text  string
	marks []mark
	// create is the replacement of the legacy root statement at removal
	// time for a root that creates its bucket: the sibling looks the
	// bucket up until then.
	create      string
	createMarks []mark
}

// planner classifies sites and assembles steps.
type planner struct {
	prog    *program
	g       *graph
	sites   []*site
	cls     []*classified
	bySite  map[*site]*classified
	answers *answerSet
}

func (p *planner) pos(n token.Pos) Position {
	ps := p.prog.fset.Position(n)
	return Position{File: p.prog.fileOf(n).rel, Line: ps.Line, Column: ps.Column}
}

func (p *planner) posID(n token.Pos) string {
	ps := p.pos(n)
	return fmt.Sprintf("%s:%d:%d", ps.File, ps.Line, ps.Column)
}

// classifyAll classifies every site: subscribe sites first, since they
// own the message uses of their handlers.
func (p *planner) classifyAll(compOf map[*site]*component) {
	p.bySite = make(map[*site]*classified)
	for _, s := range p.sites {
		c := &classified{s: s, id: p.posID(s.anchor.Pos()), comp: compOf[s]}
		p.cls = append(p.cls, c)
		p.bySite[s] = c
	}
	// Subscribe sites last: they take over the sites inside their
	// handlers.
	for _, c := range p.cls {
		if !(c.s.kind == anchorCall && table[c.s.sym].Kind == Subscribe) {
			p.classify(c)
		}
	}
	for _, c := range p.cls {
		if c.s.kind == anchorCall && table[c.s.sym].Kind == Subscribe {
			p.classifySubscribe(c)
		}
	}
}

// settle sets a site's class from the rewriter's problems and its
// decisions.
func (p *planner) settle(c *classified, rw *rewriter, intents []intent) {
	intents = append(intents, rw.extra...)
	var unm, gui []string
	for _, pr := range rw.problems {
		if pr.class == classUnmapped {
			unm = append(unm, pr.reason)
		} else {
			gui = append(gui, pr.reason)
		}
	}
	unm, gui = uniq(unm), uniq(gui)
	switch {
	case len(unm) > 0:
		c.class = classUnmapped
		c.notes = append(c.notes, unm...)
		c.facts = append(c.facts, gui...)
	case len(gui) > 0:
		c.class = classGuided
		c.facts = append(c.facts, gui...)
		c.template = p.afterText(c, intents)
		for _, t := range rw.templates {
			c.template += "\n\n" + t
		}
	case pendingDecisions(c) > 0:
		c.class = classDecision
	default:
		c.class = classMechanical
		c.intents = intents
	}
}

func uniq(s []string) []string {
	var out []string
	seen := make(map[string]bool)
	for _, x := range s {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func pendingDecisions(c *classified) int {
	n := 0
	for _, d := range c.decisions {
		if d.choice == "" {
			n++
		}
	}
	return n
}

// span returns the node a site's before and after texts show: its
// enclosing statement, or its declaration.
func (p *planner) span(c *classified) ast.Node {
	if c.s.kind == anchorDecl {
		return c.s.anchor
	}
	for i := len(c.s.stack) - 1; i >= 0; i-- {
		switch n := c.s.stack[i].(type) {
		case *ast.FuncLit:
			// A handler literal: show the whole statement around it.
		case ast.Stmt:
			if _, ok := n.(*ast.BlockStmt); ok {
				return c.s.anchor
			}
			return n
		case ast.Decl:
			return c.s.anchor
		}
	}
	return c.s.anchor
}

func (p *planner) beforeText(c *classified) string {
	return p.prog.text(p.span(c))
}

func (p *planner) afterText(c *classified, intents []intent) string {
	return p.prog.applyWithin(p.span(c), intents)
}

// classify dispatches a non-subscribe site on its anchor and symbol.
func (p *planner) classify(c *classified) {
	s := c.s
	e := table[s.sym]
	c.ref = e.Ref
	rw := &rewriter{prog: p.prog, f: s.file}
	switch s.kind {
	case anchorDecl:
		p.classifyDecl(c, rw)
	case anchorLit:
		lit := s.anchor.(*ast.CompositeLit)
		intents := rw.litIntents(lit, s.uses)
		c.links = append(c.links, litKey(p.prog, lit))
		p.valueContext(c, rw, lit)
		c.summary = fmt.Sprintf("nats.%s literal becomes jetstream.%s", s.sym, e.Target)
		p.settle(c, rw, intents)
	case anchorRef:
		if len(s.uses) == 1 && s.uses[0].value {
			p.classifyValue(c, rw)
			return
		}
		p.classifyRef(c, rw)
	case anchorCall:
		call := s.anchor.(*ast.CallExpr)
		if isRootSym(s.sym) {
			p.classifyRoot(c, rw, call)
			return
		}
		if strings.HasPrefix(s.sym, "Msg.") {
			p.classifyMsg(c, rw, call)
			return
		}
		switch e.Kind {
		case Call, Same:
			p.classifyCall(c, rw, call)
		case Rename:
			intents := rw.refIntents(call, s.uses, nil)
			p.valueContext(c, rw, call)
			c.summary = fmt.Sprintf("nats.%s becomes jetstream.%s", s.sym, e.Target)
			p.settle(c, rw, intents)
		case Guided:
			rw.guided("%s", e.Note)
			c.summary = fmt.Sprintf("%s becomes %s", s.sym, e.Target)
			p.settle(c, rw, nil)
		case Unmapped:
			rw.unmapped(e.Note)
			c.summary = fmt.Sprintf("%s has no jetstream equivalent", s.sym)
			p.settle(c, rw, nil)
		default:
			rw.guided("%s is built outside the call it configures: %s", s.sym, entryHint(s.sym, e))
			c.summary = fmt.Sprintf("%s outside a legacy call", s.sym)
			p.settle(c, rw, nil)
		}
	}
}

// classifyMsg handles a message method no subscribe site rewrites: in a
// handler the planner does not tie to its subscription, it migrates with
// that subscription; on a message from elsewhere it has no counterpart.
func (p *planner) classifyMsg(c *classified, rw *rewriter, call *ast.CallExpr) {
	s := c.s
	c.summary = fmt.Sprintf("%s on a *nats.Msg", s.sym)
	c.ref = refAck
	info := s.file.info()
	sel, _ := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	param := false
	if sel != nil {
		if id, ok := ast.Unparen(sel.X).(*ast.Ident); ok {
			if v, ok := info.Uses[id].(*types.Var); ok {
				for i := len(s.stack) - 1; i >= 0; i-- {
					var ft *ast.FuncType
					switch fn := s.stack[i].(type) {
					case *ast.FuncLit:
						ft = fn.Type
					case *ast.FuncDecl:
						ft = fn.Type
					}
					if ft != nil && ft.Params != nil {
						for _, fld := range ft.Params.List {
							for _, n := range fld.Names {
								if info.Defs[n] == v {
									param = true
								}
							}
						}
					}
				}
			}
		}
	}
	if param {
		rw.guided("the message is a handler parameter: the call migrates with the subscription that delivers it, once the handler takes a jetstream.Msg (%s)", entryHint(s.sym, table[s.sym]))
	} else {
		rw.unmapped("the message is not delivered by a legacy JetStream subscription the planner follows; the jetstream package acks only its own jetstream.Msg, so keep this call or receive the message through a jetstream consumer")
	}
	p.settle(c, rw, nil)
}

// typeUseContext checks the value a legacy type in an expression makes:
// make(T), new(T), T(x) or x.(T). The value changes type with it, and an
// asserted value must change type together with it.
func (p *planner) typeUseContext(c *classified, rw *rewriter) {
	s := c.s
	if tn, ok := s.file.info().Uses[s.uses[0].id].(*types.TypeName); ok {
		if _, handle := siblingType(tn.Type()); handle {
			rw.guided("nats.%s handles migrate through their declarations and roots; this expression makes one the planner does not thread", s.sym)
			return
		}
	}
	var cur ast.Node = s.anchor
	for i := len(s.stack) - 1; i >= 0; i-- {
		parent := s.stack[i]
		if isTypeExpr(parent, cur) || isCompositeType(parent, cur) {
			cur = parent
			continue
		}
		switch x := parent.(type) {
		case *ast.CallExpr:
			if x.Fun == cur || (len(x.Args) > 0 && x.Args[0] == cur) {
				p.valueContext(c, rw, x)
				return
			}
		case *ast.TypeAssertExpr:
			if x.Type == cur {
				p.linkOperand(c, rw, x.X)
				p.valueContext(c, rw, x)
				return
			}
		}
		rw.guided("nats.%s is used as a type in %s; change it with the values it describes", s.sym, p.prog.text(parent))
		return
	}
}

// isCompositeType reports whether child is part of a composite type
// expression parent.
func isCompositeType(parent, child ast.Node) bool {
	switch p := parent.(type) {
	case *ast.MapType:
		return p.Key == child || p.Value == child
	case *ast.ArrayType:
		return p.Elt == child
	case *ast.ChanType:
		return p.Value == child
	}
	return false
}

// errorRenames are error values a jetstream call reports under another
// name than its legacy counterpart, by legacy call.
var errorRenames = map[string]map[string]string{
	"JetStreamManager.AddConsumer": {"ErrConsumerNameAlreadyInUse": "ErrConsumerExists"},
}

// classifyValue handles a constant of a legacy type, which changes type
// with the values it is used with, or a JetStream error value, which
// migrates with the call whose error it is compared against.
func (p *planner) classifyValue(c *classified, rw *rewriter) {
	s := c.s
	u := s.uses[0]
	if _, isVar := s.file.info().Uses[u.id].(*types.Var); isVar {
		prod := p.errProducer(s)
		if prod == nil {
			// Not compared with an error of a legacy call: nothing to do.
			c.drop = true
			return
		}
		target := errorRenames[prod.s.sym][u.sym]
		it, ok := rw.valueIntent(u, target)
		if target == "" {
			target = u.sym
		}
		key := objKey("site:" + prod.id)
		c.links = append(c.links, key)
		prod.links = append(prod.links, key)
		c.summary = fmt.Sprintf("nats.%s becomes jetstream.%s with the call at %s, whose error it checks", u.sym, target, prod.id)
		var intents []intent
		if ok {
			intents = append(intents, it)
		}
		p.settle(c, rw, intents)
		return
	}
	it, ok := rw.valueIntent(u, "")
	if expr, isExpr := s.anchor.(ast.Expr); isExpr {
		p.valueContext(c, rw, expr)
	}
	c.summary = fmt.Sprintf("nats.%s becomes jetstream.%s", u.sym, u.sym)
	var intents []intent
	if ok {
		intents = append(intents, it)
	}
	p.settle(c, rw, intents)
}

// errProducer returns the site whose call produced the error a sentinel
// site is compared with: errors.Is(err, nats.ErrX), err == nats.ErrX or a
// switch case, where err was last assigned from that call.
func (p *planner) errProducer(s *site) *classified {
	info := s.file.info()
	if len(s.stack) == 0 {
		return nil
	}
	var errExpr ast.Expr
	parent := s.stack[len(s.stack)-1]
	switch x := parent.(type) {
	case *ast.CallExpr:
		if fn, ok := typeutil.Callee(info, x).(*types.Func); ok && fn.Pkg() != nil && fn.Pkg().Path() == "errors" && fn.Name() == "Is" && len(x.Args) == 2 {
			errExpr = x.Args[0]
		}
	case *ast.BinaryExpr:
		if x.Op == token.EQL || x.Op == token.NEQ {
			errExpr = x.X
			if x.X == s.anchor {
				errExpr = x.Y
			}
		}
	case *ast.CaseClause:
		for i := len(s.stack) - 2; i >= 0; i-- {
			if sw, ok := s.stack[i].(*ast.SwitchStmt); ok {
				errExpr = sw.Tag
				break
			}
		}
	}
	id, ok := ast.Unparen(errExpr).(*ast.Ident)
	if !ok {
		return nil
	}
	obj := info.Uses[id]
	if obj == nil {
		return nil
	}
	var body ast.Node
	for i := len(s.stack) - 1; i >= 0 && body == nil; i-- {
		switch fn := s.stack[i].(type) {
		case *ast.FuncLit:
			body = fn.Body
		case *ast.FuncDecl:
			body = fn.Body
		}
	}
	if body == nil {
		return nil
	}
	var last ast.Expr
	lastPos := token.NoPos
	ast.Inspect(body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || as.Pos() >= s.anchor.Pos() || len(as.Rhs) != 1 {
			return true
		}
		for _, l := range as.Lhs {
			if lid, ok := l.(*ast.Ident); ok && info.ObjectOf(lid) == obj && as.Pos() > lastPos {
				last, lastPos = as.Rhs[0], as.Pos()
			}
		}
		return true
	})
	call, ok := ast.Unparen(last).(*ast.CallExpr)
	if !ok {
		return nil
	}
	for _, c := range p.cls {
		if c.s.anchor == call {
			return c
		}
	}
	return nil
}

// classifyRef handles a legacy reference outside calls, literals and
// declarations: a constant, error value or type in an expression.
func (p *planner) classifyRef(c *classified, rw *rewriter) {
	s := c.s
	e := table[s.sym]
	switch e.Kind {
	case Rename:
		intents := rw.refIntents(s.anchor, s.uses, nil)
		if len(s.uses) > 0 {
			if _, isType := s.file.info().Uses[s.uses[0].id].(*types.TypeName); isType {
				p.typeUseContext(c, rw)
			} else if expr, ok := s.anchor.(ast.Expr); ok {
				p.valueContext(c, rw, expr)
			}
		}
		c.summary = fmt.Sprintf("nats.%s becomes jetstream.%s", s.sym, e.Target)
		p.settle(c, rw, intents)
	case Unmapped:
		rw.unmapped(e.Note)
		c.summary = fmt.Sprintf("%s has no jetstream equivalent", s.sym)
		p.settle(c, rw, nil)
	default:
		rw.guided("%s is used as a value: %s", s.sym, entryHint(s.sym, e))
		c.summary = fmt.Sprintf("%s used as a value", s.sym)
		p.settle(c, rw, nil)
	}
}

// classifyDecl handles a declaration whose type names a legacy type: a
// handle declaration gets a sibling; any other is retyped with its value.
func (p *planner) classifyDecl(c *classified, rw *rewriter) {
	s := c.s
	info := s.file.info()
	names, typ := declNames(s.anchor)
	if typ == nil {
		rw.guided("a legacy type is declared or aliased here; rewrite the declaration by hand")
		c.summary = "type declaration over a legacy type"
		p.settle(c, rw, nil)
		return
	}
	if nt, ok := siblingType(info.TypeOf(typ)); ok {
		c.role = roleDecl
		var h *handle
		for _, n := range names {
			if hh, ok := p.g.handles[p.prog.key(info.Defs[n])]; ok {
				h = hh
			}
		}
		switch {
		case h == nil:
			rw.guided("an unnamed %s declaration; name it to thread a %s next to it", s.sym, nt)
		case len(names) != 1:
			rw.guided("the handle is declared together with other names; declare it on its own")
		case !h.threadable:
			rw.guided("the handle cannot be threaded: %s", h.why)
		case !p.declThreadable(s):
			rw.guided("the handle is a parameter of a function type or interface method, whose implementations would all change")
		}
		c.summary = fmt.Sprintf("declare a %s sibling next to the nats.%s handle", nt, s.sym)
		if h != nil {
			c.summary = fmt.Sprintf("declare %s %s next to %s", h.sibling, nt, h.ident.Name)
			sep := "\n"
			if h.kind == handleParam {
				sep = ", "
			}
			c.after = p.prog.text(s.anchor) + sep + h.sibling + " " + nt
		}
		c.ref = refInit
		p.settle(c, rw, nil)
		return
	}
	if containsHandle(info.TypeOf(typ)) {
		rw.guided("a container of legacy handles; migrate the handles it holds by hand")
		c.summary = "container of legacy handles"
		p.settle(c, rw, nil)
		return
	}
	intents := rw.refIntents(typ, s.uses, nil)
	if iface := p.implementedLegacy(s); iface != "" {
		rw.guided("the method belongs to an implementation of nats.%s, whose signature would no longer match; migrate the implementation with the interface (regenerate a mock from jetstream.%s)", iface, table[iface].Target)
	}
	for _, n := range names {
		if obj := info.Defs[n]; obj != nil {
			c.links = append(c.links, p.prog.key(obj))
		}
	}
	if len(names) == 0 {
		if v := unnamedVar(info, s.anchor); v != nil {
			c.links = append(c.links, p.prog.key(v))
		}
	}
	if vs, ok := s.anchor.(*ast.ValueSpec); ok && len(vs.Values) == 0 && isConsumerConfig(info.TypeOf(typ)) {
		rw.guided("a zero legacy ConsumerConfig has AckPolicy AckNonePolicy, a zero jetstream.ConsumerConfig AckExplicitPolicy; set AckPolicy: jetstream.AckNonePolicy to keep the behavior")
	}
	c.summary = fmt.Sprintf("retype nats.%s as jetstream.%s", s.sym, table[s.sym].Target)
	p.settle(c, rw, intents)
}

// implementedLegacy returns the legacy interface a method declaration's
// receiver type implements, when the declaration is a parameter or result
// of that method.
func (p *planner) implementedLegacy(s *site) string {
	var fd *ast.FuncDecl
	for i := len(s.stack) - 1; i >= 0; i-- {
		if d, ok := s.stack[i].(*ast.FuncDecl); ok {
			fd = d
			break
		}
		if _, ok := s.stack[i].(*ast.BlockStmt); ok {
			return ""
		}
	}
	if fd == nil || fd.Recv == nil {
		return ""
	}
	fn, ok := s.file.info().Defs[fd.Name].(*types.Func)
	if !ok {
		return ""
	}
	recv := fn.Signature().Recv().Type()
	if pt, ok := recv.(*types.Pointer); ok {
		recv = pt.Elem()
	}
	for _, iface := range p.legacyInterfaces(s.file) {
		it := iface.Type().Underlying().(*types.Interface)
		if types.Implements(recv, it) || types.Implements(types.NewPointer(recv), it) {
			return iface.Name()
		}
	}
	return ""
}

// legacyInterfaces returns the legacy interface types of the nats package
// f imports.
func (p *planner) legacyInterfaces(f *srcFile) []*types.TypeName {
	var nats *types.Package
	for _, imp := range f.pkg.Types.Imports() {
		if imp.Path() == natsModule {
			nats = imp
		}
	}
	if nats == nil {
		return nil
	}
	var out []*types.TypeName
	for _, name := range nats.Scope().Names() {
		tn, ok := nats.Scope().Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		if _, ok := tn.Type().Underlying().(*types.Interface); !ok {
			continue
		}
		if e, ok := table[name]; ok && e.Kind == Rename {
			out = append(out, tn)
		}
	}
	return out
}

func isConsumerConfig(t types.Type) bool {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	sym, ok := legacyKey(n.Obj())
	return ok && sym == "ConsumerConfig"
}

// declNames returns the names and the type expression of a declaration
// anchor; a nil type means a type declaration.
func declNames(n ast.Node) ([]*ast.Ident, ast.Expr) {
	switch d := n.(type) {
	case *ast.Field:
		return d.Names, d.Type
	case *ast.ValueSpec:
		return d.Names, d.Type
	}
	return nil, nil
}

// unnamedVar returns the variable of an unnamed parameter or result.
func unnamedVar(info *types.Info, n ast.Node) *types.Var {
	fld, ok := n.(*ast.Field)
	if !ok {
		return nil
	}
	var found *types.Var
	for _, obj := range info.Defs {
		if fn, ok := obj.(*types.Func); ok {
			sig := fn.Type().(*types.Signature)
			for _, tup := range []*types.Tuple{sig.Params(), sig.Results()} {
				for i := range tup.Len() {
					if tup.At(i).Pos() == fld.Type.Pos() {
						found = tup.At(i)
					}
				}
			}
		}
	}
	return found
}

func containsHandle(t types.Type) bool {
	switch u := t.(type) {
	case *types.Pointer:
		return containsHandle(u.Elem())
	case *types.Slice:
		return containsHandle(u.Elem())
	case *types.Array:
		return containsHandle(u.Elem())
	case *types.Map:
		return containsHandle(u.Key()) || containsHandle(u.Elem())
	case *types.Chan:
		return containsHandle(u.Elem())
	}
	_, ok := siblingType(t)
	return ok
}

// declThreadable reports whether a sibling parameter can be added next to
// a handle parameter: the function has a body and is only ever called.
func (p *planner) declThreadable(s *site) bool {
	fld, ok := s.anchor.(*ast.Field)
	if !ok {
		return true
	}
	for i := len(s.stack) - 1; i >= 0; i-- {
		switch n := s.stack[i].(type) {
		case *ast.StructType:
			return true
		case *ast.FuncType:
			if i == 0 {
				return false
			}
			switch parent := s.stack[i-1].(type) {
			case *ast.FuncDecl:
				if parent.Type != n {
					return false
				}
				return fld != nil && p.onlyCalled(s.file.info().Defs[parent.Name])
			case *ast.FuncLit:
				return false
			default:
				return false
			}
		}
	}
	return true
}

// onlyCalled reports whether every use of a function is a call.
func (p *planner) onlyCalled(obj types.Object) bool {
	if obj == nil {
		return false
	}
	k := p.prog.key(obj)
	for _, f := range p.prog.files {
		info := f.info()
		ok := true
		ast.Inspect(f.ast, func(n ast.Node) bool {
			if !ok {
				return false
			}
			switch x := n.(type) {
			case *ast.CallExpr:
				// Visit arguments, not the callee.
				var fun ast.Node = ast.Unparen(x.Fun)
				if sel, isSel := fun.(*ast.SelectorExpr); isSel {
					ast.Inspect(sel.X, func(n ast.Node) bool {
						if id, isID := n.(*ast.Ident); isID && p.prog.key(info.Uses[id]) == k {
							ok = false
						}
						return ok
					})
				}
				for _, a := range x.Args {
					ast.Inspect(a, func(n ast.Node) bool {
						if id, isID := n.(*ast.Ident); isID && p.prog.key(info.Uses[id]) == k {
							ok = false
						}
						return ok
					})
				}
				return false
			case *ast.Ident:
				if p.prog.key(info.Uses[x]) == k {
					ok = false
				}
			}
			return true
		})
		if !ok {
			return false
		}
	}
	return true
}

// recvHandle resolves the receiver of a legacy method call on a handle.
func (p *planner) recvHandle(rw *rewriter, recv ast.Expr) (*handle, bool) {
	info := rw.f.info()
	if _, ok := siblingType(info.TypeOf(recv)); !ok {
		return nil, false
	}
	h, ok := p.g.handleOf(info, recv)
	if !ok {
		rw.guided("the handle is reached through an expression, not a variable or field the planner threads")
		return nil, true
	}
	if !h.threadable {
		rw.guided("the handle cannot be threaded: %s", h.why)
	}
	return h, true
}

// siblingRecv writes the sibling expression of a handle receiver.
func (p *planner) siblingRecv(b *builder, recv ast.Expr, h *handle) {
	if sel, ok := ast.Unparen(recv).(*ast.SelectorExpr); ok {
		b.add(p.prog.text(sel.X), ".")
	}
	b.sib(h)
}

// optionIntent maps one legacy option call to its jetstream counterpart,
// or records why it cannot be.
type optResult struct {
	text    string
	ctx     ast.Expr // the argument of a nats.Context option
	dropped bool
}

func (p *planner) mapOption(rw *rewriter, opt ast.Expr, perCall bool, elem types.Type, handled map[*ast.Ident]bool) optResult {
	info := rw.f.info()
	call, ok := ast.Unparen(opt).(*ast.CallExpr)
	if !ok {
		rw.guided("option %s is not a direct call of a legacy option", p.prog.text(opt))
		return optResult{}
	}
	sym, ok := calleeKey(info, call)
	if !ok {
		rw.guided("option %s is not a legacy option", p.prog.text(opt))
		return optResult{}
	}
	markCallee(call, handled)
	e := table[sym]
	switch {
	case sym == "Context" || sym == "ContextOpt":
		if len(call.Args) == 1 {
			return optResult{ctx: call.Args[0], dropped: true}
		}
	case e.Kind == Unmapped:
		rw.unmapped(fmt.Sprintf("%s: %s", sym, e.Note))
		return optResult{}
	case sym == "MaxWait" && perCall:
		rw.guided("a per-call nats.MaxWait becomes a context with that timeout: ctx, cancel := context.WithTimeout(ctx, d); defer cancel()")
		return optResult{}
	case sym == "AckWait" && perCall:
		rw.guided("a per-call nats.AckWait becomes a context with that timeout: ctx, cancel := context.WithTimeout(ctx, d); defer cancel()")
		return optResult{}
	case e.Kind == Option && !strings.HasPrefix(e.Target, "NewWith"):
		fn, _ := p.prog.jsObject(e.Target).(*types.Func)
		if fn == nil {
			rw.guided("option %s has no jetstream counterpart", sym)
			return optResult{}
		}
		if elem != nil {
			res := fn.Type().(*types.Signature).Results()
			if res.Len() != 1 || !types.AssignableTo(res.At(0).Type(), elem) {
				rw.guided("option %s has no counterpart for this call", sym)
				return optResult{}
			}
		}
		args := make([]string, len(call.Args))
		for i, a := range call.Args {
			args[i] = p.exprText(rw, a, handled)
		}
		return optResult{text: "jetstream." + e.Target + "(" + strings.Join(args, ", ") + ")"}
	}
	rw.guided("option %s: %s", sym, entryHint(sym, e))
	return optResult{}
}

// markCallee marks the identifiers of a call's callee as handled.
func markCallee(call *ast.CallExpr, handled map[*ast.Ident]bool) {
	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.Ident:
		handled[fun] = true
	case *ast.SelectorExpr:
		handled[fun.Sel] = true
	}
}

// exprText returns the text of e with the legacy references and literals
// inside it rewritten, marking the uses it rewrites.
func (p *planner) exprText(rw *rewriter, e ast.Expr, handled map[*ast.Ident]bool) string {
	var intents []intent
	uses := p.usesIn(rw.f, e)
	var lits []*ast.CompositeLit
	ast.Inspect(e, func(n ast.Node) bool {
		if lit, ok := n.(*ast.CompositeLit); ok {
			if t := rw.f.info().TypeOf(lit); t != nil && hasLegacyType(t) {
				lits = append(lits, lit)
				return false
			}
		}
		return true
	})
	for _, lit := range lits {
		intents = append(intents, rw.litIntents(lit, uses)...)
		for _, u := range uses {
			if u.id.Pos() >= lit.Pos() && u.id.End() <= lit.End() {
				handled[u.id] = true
			}
		}
	}
	inLit := func(id *ast.Ident) bool {
		for _, lit := range lits {
			if id.Pos() >= lit.Pos() && id.End() <= lit.End() {
				return true
			}
		}
		return false
	}
	var rest []legacyUse
	for _, u := range uses {
		if !inLit(u.id) && !handled[u.id] {
			rest = append(rest, u)
			handled[u.id] = true
		}
	}
	intents = append(intents, rw.refIntents(e, rest, nil)...)
	return p.prog.applyWithin(e, intents)
}

// usesIn returns the legacy uses inside n.
func (p *planner) usesIn(f *srcFile, n ast.Node) []legacyUse {
	var out []legacyUse
	ast.Inspect(n, func(m ast.Node) bool {
		if id, ok := m.(*ast.Ident); ok {
			if sym, ok := legacyKey(f.info().Uses[id]); ok {
				out = append(out, legacyUse{id: id, sym: sym})
			} else if sym, ok := p.prog.valueRef(f.info().Uses[id]); ok {
				out = append(out, legacyUse{id: id, sym: sym, value: true})
			}
		}
		return true
	})
	return out
}

// classifyCall rewrites a legacy method call: on a handle, through its
// sibling; on a value, in place.
func (p *planner) classifyCall(c *classified, rw *rewriter, call *ast.CallExpr) {
	s := c.s
	sym := s.sym
	e := table[sym]
	handled := make(map[*ast.Ident]bool)
	intents, summary := p.callRewrite(c, rw, call, sym, e, handled)
	c.summary = summary
	for _, d := range s.dependents {
		intents = append(intents, p.dependentRewrite(c, rw, d, handled)...)
	}
	p.checkHandled(rw, s, handled, intents)
	p.settle(c, rw, intents)
}

// checkHandled fails the site when a legacy use inside it was neither
// consumed nor renamed.
func (p *planner) checkHandled(rw *rewriter, s *site, handled map[*ast.Ident]bool, intents []intent) {
	for _, u := range s.uses {
		if handled[u.id] || table[u.sym].Kind == Same {
			continue
		}
		covered := false
		off := p.prog.offset(u.id.Pos())
		for _, it := range intents {
			if it.file == s.file && off >= it.start && off < it.end {
				covered = true
			}
		}
		if !covered {
			rw.guided("%s is not rewritten by the planner here", u.sym)
		}
	}
}

// callRewrite builds the replacement of one legacy method call.
func (p *planner) callRewrite(c *classified, rw *rewriter, call *ast.CallExpr, sym string, e Entry, handled map[*ast.Ident]bool) ([]intent, string) {
	info := rw.f.info()
	markCallee(call, handled)
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		rw.guided("%s is not called as a method", sym)
		return nil, sym
	}
	legacyFn, _ := typeutil.Callee(info, call).(*types.Func)
	if legacyFn == nil {
		rw.guided("%s is called through a function value", sym)
		return nil, sym
	}
	lsig := legacyFn.Type().(*types.Signature)
	if push, ok := pushTargets[sym]; ok && len(call.Args) >= 2 {
		// A config with a deliver subject makes a push consumer.
		if lit := configArgLit(call.Args[1]); litField(lit, "DeliverSubject") != nil {
			e.Target = push
		}
	}
	_, member, _ := strings.Cut(e.Target, ".")
	summary := fmt.Sprintf("%s becomes %s", sym, e.Target)
	tsig := p.prog.jsSignature(e.Target)
	if tsig == nil {
		rw.guided("%s has no jetstream counterpart in this nats.go", e.Target)
		return nil, summary
	}
	b := &builder{}
	h, isHandle := p.recvHandle(rw, sel.X)
	if isHandle && h == nil {
		return nil, summary
	}
	if !isHandle && e.Kind == Same && member == sel.Sel.Name && (tsig.Params().Len() == 0 || !isContext(tsig.Params().At(0).Type())) {
		// The method is unchanged on the retyped value: only the
		// arguments may need rewriting, and the receiver is left to the
		// sites inside it.
		var out []intent
		for _, a := range call.Args {
			if txt := p.exprText(rw, a, handled); txt != p.prog.text(a) {
				out = append(out, p.prog.replace(a, txt))
			}
		}
		p.resultContext(c, rw, call, fitResults(lsig.Results(), tupleTypes(tsig.Results())))
		return out, summary
	}
	recvText := func(b *builder) {
		if isHandle {
			p.siblingRecv(b, sel.X, h)
		} else {
			b.add(p.prog.text(sel.X))
		}
	}
	// Split fixed arguments from options.
	args := call.Args
	nfixed := len(args)
	if lsig.Variadic() {
		nfixed = lsig.Params().Len() - 1
		if call.Ellipsis.IsValid() {
			rw.guided("options are passed as a slice; map each option to its jetstream counterpart")
			return nil, summary
		}
	}
	if nfixed > len(args) {
		nfixed = len(args)
	}
	fixed, opts := args[:nfixed], args[nfixed:]
	var elem types.Type
	if tsig.Variadic() {
		elem = tsig.Params().At(tsig.Params().Len() - 1).Type().(*types.Slice).Elem()
	}
	var ctxExpr ast.Expr
	var newOpts []string
	for _, o := range opts {
		r := p.mapOption(rw, o, true, elem, handled)
		if r.ctx != nil {
			ctxExpr = r.ctx
		}
		if r.text != "" {
			newOpts = append(newOpts, r.text)
		}
	}
	ctxText := ""
	ctx := func() string {
		if ctxText == "" {
			if ctxExpr != nil {
				ctxText = p.exprText(rw, ctxExpr, handled)
			} else {
				ctxText, _ = p.prog.ctxAt(rw.f, call.Pos(), nil)
			}
		}
		return ctxText
	}
	shape := e.Shape
	via := shape&(ViaStream|ViaConsumer) != 0
	// Map fixed arguments onto the target's parameters.
	tfixed := tsig.Params().Len()
	if tsig.Variadic() {
		tfixed--
	}
	tparams := make([]types.Type, 0, tfixed)
	for i := range tfixed {
		tparams = append(tparams, tsig.Params().At(i).Type())
	}
	if len(tparams) > 0 && isContext(tparams[0]) {
		tparams = tparams[1:]
	}
	lead := 0
	switch {
	case shape&ViaStream != 0:
		lead = 1
	case shape&ViaConsumer != 0:
		lead = 2
	}
	if len(fixed) < lead || len(fixed)-lead != len(tparams) {
		rw.guided("%s takes different arguments than %s", sym, e.Target)
		return nil, summary
	}
	var fixedText []string
	for i, a := range fixed {
		txt := p.exprText(rw, a, handled)
		if i >= lead {
			txt = p.adaptArg(rw, a, txt, lsig.Params().At(i).Type(), tparams[i-lead])
		}
		fixedText = append(fixedText, txt)
	}
	// Results.
	var newResults []types.Type
	switch {
	case e.Accessor != "":
		lister, ok := tsig.Results().At(0).Type().(*types.Named)
		asig := (*types.Signature)(nil)
		if ok {
			asig = p.prog.jsSignature(lister.Obj().Name() + "." + e.Accessor)
		}
		if asig == nil {
			rw.guided("%s has no %s accessor", e.Target, e.Accessor)
			return nil, summary
		}
		newResults = tupleTypes(asig.Results())
		c.followUps = append(c.followUps, FollowUp{Kind: "lister-error", Summary: fmt.Sprintf("check the error of the %s lister after ranging over %s()", member, e.Accessor)})
	default:
		newResults = tupleTypes(tsig.Results())
	}
	fits := fitResults(lsig.Results(), newResults)
	p.resultContext(c, rw, call, fits)
	if e.Note != "" && e.Kind != Guided {
		c.notes = append(c.notes, e.Note)
	}
	// Build the text.
	callArgs := func(withCtx bool, rest []string) string {
		var all []string
		if withCtx {
			all = append(all, ctx())
		}
		all = append(all, rest...)
		all = append(all, newOpts...)
		return strings.Join(all, ", ")
	}
	addsCtx := tsig.Params().Len() > 0 && isContext(tsig.Params().At(0).Type())
	if !via {
		recvText(b)
		b.add(".", member, "(", callArgs(addsCtx, fixedText), ")")
		if e.Accessor != "" {
			b.add(".", e.Accessor, "()")
		}
		return []intent{b.at(rw.f, p.prog.offset(call.Pos()), p.prog.offset(call.End()))}, summary
	}
	// Through a stream or consumer handle: an immediately called function
	// literal keeps the call an expression with the legacy results.
	avoid := idents(call)
	hv := fresh("stream", avoid)
	getter, getArgs := "Stream", fixedText[:1]
	if shape&ViaConsumer != 0 {
		hv = fresh("consumer", avoid)
		getter, getArgs = "Consumer", fixedText[:2]
	}
	errv := fresh("err", avoid)
	resTypes := tupleTypes(lsig.Results())
	var resText []string
	for _, t := range resTypes {
		resText = append(resText, jsTypeText(t))
	}
	results := strings.Join(resText, ", ")
	if len(resText) > 1 {
		results = "(" + results + ")"
	}
	zero := append(zeroResults(resTypes), errv)
	indent := p.prog.indentOf(call.Pos())
	b.add("func() ", results, " {\n", indent, "\t", hv, ", ", errv, " := ")
	recvText(b)
	b.add(".", getter, "(", ctx(), ", ", strings.Join(getArgs, ", "), ")\n")
	b.add(indent, "\tif ", errv, " != nil {\n", indent, "\t\treturn ", strings.Join(zero, ", "), "\n", indent, "\t}\n")
	var rest []string
	rest = append(rest, fixedText[lead:]...)
	b.add(indent, "\treturn ", hv, ".", member, "(", callArgs(addsCtx, rest), ")\n", indent, "}()")
	return []intent{b.at(rw.f, p.prog.offset(call.Pos()), p.prog.offset(call.End()))}, summary
}

// pushTargets are the push counterparts of consumer management calls.
var pushTargets = map[string]string{
	"JetStreamManager.AddConsumer":    "JetStream.CreatePushConsumer",
	"JetStreamManager.UpdateConsumer": "JetStream.UpdatePushConsumer",
}

// configArgLit returns the literal a config argument is or points to.
func configArgLit(e ast.Expr) *ast.CompositeLit {
	e = ast.Unparen(e)
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		e = ast.Unparen(u.X)
	}
	lit, _ := e.(*ast.CompositeLit)
	return lit
}

// adaptArg converts an argument to the target parameter's type: a
// pointer to a legacy struct passed where jetstream takes the value.
func (p *planner) adaptArg(rw *rewriter, a ast.Expr, txt string, old, target types.Type) string {
	if mappedTypeKey(old) == typeKey(target) {
		return txt
	}
	if ptr, ok := old.(*types.Pointer); ok && mappedTypeKey(ptr.Elem()) == typeKey(target) {
		if u, ok := ast.Unparen(a).(*ast.UnaryExpr); ok && u.Op == token.AND {
			return p.exprText(rw, u.X, map[*ast.Ident]bool{})
		}
		switch ast.Unparen(a).(type) {
		case *ast.Ident, *ast.SelectorExpr:
			return "*" + txt
		}
		return "*(" + txt + ")"
	}
	rw.guided("argument %s has type %s where the jetstream call takes %s", p.prog.text(a), jsTypeText(old), jsTypeText(target))
	return txt
}

// dependentRewrite rewrites a legacy call on a value another site
// produces.
func (p *planner) dependentRewrite(c *classified, rw *rewriter, d *site, handled map[*ast.Ident]bool) []intent {
	call, ok := d.anchor.(*ast.CallExpr)
	if !ok {
		return nil
	}
	e := table[d.sym]
	switch e.Kind {
	case Same:
		markCallee(call, handled)
		var out []intent
		for _, a := range call.Args {
			txt := p.exprText(rw, a, handled)
			if txt != p.prog.text(a) {
				out = append(out, p.prog.replace(a, txt))
			}
		}
		var newResults []types.Type
		if tsig := p.prog.jsSignature(e.Target); tsig != nil {
			newResults = tupleTypes(tsig.Results())
		}
		p.resultContext(c, rw, call, fitResults(typeutil.Callee(rw.f.info(), call).Type().(*types.Signature).Results(), newResults))
		return out
	case Call:
		out, _ := p.callRewrite(c, rw, call, d.sym, e, handled)
		return out
	case Guided:
		markCallee(call, handled)
		rw.guided("%s: %s", d.sym, e.Note)
		if d.sym == "Subscription.Fetch" || d.sym == "Subscription.FetchBatch" {
			rw.templates = append(rw.templates, p.fetchTemplate(rw, call, handled))
		}
	case Unmapped:
		markCallee(call, handled)
		rw.unmapped(fmt.Sprintf("%s: %s", d.sym, e.Note))
	default:
		markCallee(call, handled)
		rw.guided("%s on a value this site produces: %s", d.sym, entryHint(d.sym, e))
	}
	return nil
}

// fetchTemplate shows the jetstream shape of a legacy Fetch loop.
func (p *planner) fetchTemplate(rw *rewriter, call *ast.CallExpr, handled map[*ast.Ident]bool) string {
	sel := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	args := []string{}
	if len(call.Args) > 0 {
		args = append(args, p.prog.text(call.Args[0]))
	}
	for _, o := range call.Args[1:] {
		oc, ok := ast.Unparen(o).(*ast.CallExpr)
		if !ok {
			continue
		}
		sym, _ := calleeKey(rw.f.info(), oc)
		markCallee(oc, handled)
		var a []string
		for _, x := range oc.Args {
			a = append(a, p.prog.text(x))
		}
		switch sym {
		case "MaxWait":
			args = append(args, "jetstream.FetchMaxWait("+strings.Join(a, ", ")+")")
		case "PullHeartbeat":
			args = append(args, "jetstream.FetchHeartbeat("+strings.Join(a, ", ")+")")
		case "Context", "ContextOpt":
			args = append(args, "jetstream.FetchContext("+strings.Join(a, ", ")+")")
		default:
			args = append(args, "/* "+p.prog.text(oc)+" */")
		}
	}
	return "batch, err := " + p.prog.text(sel.X) + ".Fetch(" + strings.Join(args, ", ") + ")\n" +
		"if err != nil {\n\t// the pull request failed; an empty batch is not an error\n}\n" +
		"for msg := range batch.Messages() {\n\t// the loop body, with msg a jetstream.Msg\n}\n" +
		"if err := batch.Error(); err != nil {\n\t// the batch ended with an error\n}"
}

// resultContext checks how a rewritten call's results are used: results
// of a different type must be discarded; retyped results may define new
// variables, which the site then links.
func (p *planner) resultContext(c *classified, rw *rewriter, call *ast.CallExpr, fits []resultFit) {
	worst := fitSame
	for _, f := range fits {
		worst = max(worst, f)
	}
	if worst == fitSame && fits != nil {
		return
	}
	info := rw.f.info()
	path, _ := astutil.PathEnclosingInterval(rw.f.ast, call.Pos(), call.End())
	parent := parentOf(path, call)
	switch x := parent.(type) {
	case *ast.ExprStmt, *ast.DeferStmt, *ast.GoStmt:
		return
	case *ast.AssignStmt:
		if len(x.Rhs) != 1 || ast.Unparen(x.Rhs[0]) != call {
			break
		}
		for i, l := range x.Lhs {
			fit := fitRetyped
			if fits != nil && i < len(fits) {
				fit = fits[i]
			}
			if fit == fitSame {
				continue
			}
			id, ok := l.(*ast.Ident)
			if ok && id.Name == "_" {
				continue
			}
			if fit == fitDifferent {
				rw.guided("the call's result %s changes to a different type in the jetstream API", p.prog.text(l))
				continue
			}
			if !ok || x.Tok != token.DEFINE || info.Defs[id] == nil {
				p.linkLHS(c, rw, l)
				continue
			}
			p.addLink(c, rw, info.Defs[id], l)
		}
		return
	case *ast.ValueSpec:
		for i, n := range x.Names {
			fit := fitRetyped
			if fits != nil && i < len(fits) {
				fit = fits[i]
			}
			if fit == fitDifferent && n.Name != "_" {
				rw.guided("the call's result %s changes to a different type in the jetstream API", n.Name)
			} else if n.Name != "_" {
				p.addLink(c, rw, info.Defs[n], n)
			}
		}
		return
	case *ast.IfStmt, *ast.SwitchStmt:
	}
	if worst == fitDifferent {
		rw.guided("the call's results change type in the jetstream API; they are used directly here")
		return
	}
	p.valueContext(c, rw, call)
}

// linkLHS links a site to what an assignment's left-hand side stores
// into: a variable, the variable holding a legacy struct whose field it
// is, or the container of an element.
func (p *planner) linkLHS(c *classified, rw *rewriter, e ast.Expr) {
	info := rw.f.info()
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		p.addLink(c, rw, info.ObjectOf(x), x)
	case *ast.SelectorExpr:
		obj := info.ObjectOf(x.Sel)
		if v, ok := obj.(*types.Var); ok && v.IsField() && !p.prog.inLoaded(v) && hasLegacyType(info.TypeOf(x.X)) {
			// A field of a legacy struct changes type with the struct.
			p.linkLHS(c, rw, x.X)
			return
		}
		p.addLink(c, rw, obj, x)
	case *ast.IndexExpr:
		p.linkLHS(c, rw, x.X)
	case *ast.StarExpr:
		p.linkLHS(c, rw, x.X)
	default:
		rw.guided("the value is stored into %s, whose type the planner cannot change", p.prog.text(e))
	}
}

// addLink links a site to a variable its value flows into; a variable
// the planner cannot retype (declared outside the loaded code, or not of
// the legacy type) needs review.
func (p *planner) addLink(c *classified, rw *rewriter, obj types.Object, at ast.Node) {
	if obj == nil {
		rw.guided("the value flows into %s, whose type the planner cannot change", p.prog.text(at))
		return
	}
	if obj.Name() == "_" {
		return
	}
	if p.isLegacyVar(obj) {
		c.links = append(c.links, p.prog.key(obj))
		return
	}
	if _, handle := siblingType(obj.Type()); handle {
		rw.guided("the value flows into the legacy handle %s", obj.Name())
		return
	}
	if _, ok := obj.Type().Underlying().(*types.Interface); ok {
		return
	}
	rw.guided("the value flows into %s, whose type the planner cannot change", obj.Name())
}

func parentOf(path []ast.Node, n ast.Node) ast.Node {
	for i, m := range path {
		if m == n && i+1 < len(path) {
			for j := i + 1; j < len(path); j++ {
				if _, ok := path[j].(*ast.ParenExpr); !ok {
					return path[j]
				}
			}
		}
	}
	return nil
}

// valueContext checks where an expression whose type changes ends up: a
// variable (linked), a nil comparison, an interface argument or a
// selector are fine; anything else needs review.
func (p *planner) valueContext(c *classified, rw *rewriter, e ast.Expr) {
	info := rw.f.info()
	t := info.TypeOf(e)
	if t == nil || !hasLegacyType(t) {
		return
	}
	path, _ := astutil.PathEnclosingInterval(rw.f.ast, e.Pos(), e.End())
	var cur ast.Node = e
	for {
		parent := parentOf(path, cur)
		switch x := parent.(type) {
		case *ast.UnaryExpr, *ast.StarExpr:
			cur = x
			continue
		case *ast.IndexExpr:
			if x.X == cur {
				// An element changes type with its container.
				cur = x
				continue
			}
		case *ast.SelectorExpr:
			if x.X == cur && p.fieldSelection(rw, x) {
				cur = x
				continue
			}
			return
		case *ast.ExprStmt:
			return
		case *ast.AssignStmt:
			if slices.Contains(x.Lhs, ast.Expr(cur.(ast.Expr))) {
				// Stored into: the right-hand side's own context links it.
				return
			}
			for i, r := range x.Rhs {
				if r == cur && i < len(x.Lhs) {
					p.linkLHS(c, rw, x.Lhs[i])
					return
				}
			}
			if len(x.Rhs) == 1 && ast.Unparen(x.Rhs[0]) == cur {
				for _, l := range x.Lhs {
					p.linkLHS(c, rw, l)
				}
				return
			}
		case *ast.ValueSpec:
			for i, v := range x.Values {
				if v == cur && i < len(x.Names) {
					p.addLink(c, rw, info.Defs[x.Names[i]], x.Names[i])
					return
				}
			}
		case *ast.KeyValueExpr:
			if x.Value == cur {
				if key, ok := x.Key.(*ast.Ident); ok {
					if obj := info.Uses[key]; obj != nil && p.prog.inLoaded(obj) {
						p.addLink(c, rw, obj, key)
						return
					}
				}
				// A field of a legacy literal: the literal's own site.
				lit := parentOf(path, x)
				if cl, ok := lit.(*ast.CompositeLit); ok && hasLegacyType(info.TypeOf(cl)) {
					c.links = append(c.links, litKey(p.prog, cl))
					return
				}
			}
		case *ast.CompositeLit:
			if hasLegacyType(info.TypeOf(x)) {
				c.links = append(c.links, litKey(p.prog, x))
				return
			}
		case *ast.BinaryExpr:
			if (x.Op == token.EQL || x.Op == token.NEQ) && (isNil(x.X) || isNil(x.Y)) {
				return
			}
			if x.Op == token.EQL || x.Op == token.NEQ {
				other := x.X
				if other == cur {
					other = x.Y
				}
				p.linkOperand(c, rw, other)
				return
			}
		case *ast.CallExpr:
			if p.argContext(c, rw, x, cur) {
				return
			}
		case *ast.ReturnStmt:
			if v := p.resultVar(rw, path, x, cur); v != nil {
				p.addLink(c, rw, v, x)
				return
			}
		case *ast.CaseClause:
			for i := len(path) - 1; i >= 0; i-- {
				if sw, ok := path[i].(*ast.SwitchStmt); ok && sw.Tag != nil && slices.Contains(sw.Body.List, ast.Stmt(x)) {
					p.linkOperand(c, rw, sw.Tag)
				}
			}
			return
		case *ast.SwitchStmt:
			return
		}
		old := types.TypeString(t, func(p *types.Package) string { return p.Name() })
		rw.guided("%s changes type from %s to %s; review its use in %s", p.prog.text(e), old, legacyTypeText(old), p.prog.text(parentStmt(path, cur)))
		return
	}
}

// linkOperand links a site to the legacy-typed values an operand it is
// compared with mentions, so that both sides change type together.
func (p *planner) linkOperand(c *classified, rw *rewriter, e ast.Expr) {
	info := rw.f.info()
	ast.Inspect(e, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.Ident:
			if obj := info.ObjectOf(x); p.isLegacyVar(obj) {
				c.links = append(c.links, p.prog.key(obj))
			}
		case *ast.CallExpr:
			if fn, ok := typeutil.Callee(info, x).(*types.Func); ok && p.prog.inLoaded(fn) {
				res := fn.Type().(*types.Signature).Results()
				for i := range res.Len() {
					if p.isLegacyVar(res.At(i)) {
						c.links = append(c.links, p.prog.key(res.At(i)))
					}
				}
			}
			for _, o := range p.cls {
				if o.s.anchor == ast.Node(x) {
					c.links = append(c.links, objKey("site:"+o.id))
					o.links = append(o.links, objKey("site:"+o.id))
				}
			}
		}
		return true
	})
}

// litKey identifies a legacy composite literal site by position.
func litKey(prog *program, lit *ast.CompositeLit) objKey {
	return objKey(fmt.Sprintf("lit:%s", prog.fset.Position(lit.Pos())))
}

func isNil(e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && id.Name == "nil"
}

func parentStmt(path []ast.Node, n ast.Node) ast.Node {
	for i, m := range path {
		if m == n {
			for _, up := range path[i:] {
				if st, ok := up.(ast.Stmt); ok {
					return st
				}
			}
		}
	}
	return n
}

// argContext handles a retyped value passed as an argument: a parameter
// of a loaded function is linked, an interface parameter accepts it.
func (p *planner) argContext(c *classified, rw *rewriter, call *ast.CallExpr, arg ast.Node) bool {
	info := rw.f.info()
	idx := slices.IndexFunc(call.Args, func(a ast.Expr) bool { return a == arg })
	if idx < 0 {
		return false
	}
	if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok {
		if b, ok := info.Uses[id].(*types.Builtin); ok && (b.Name() == "len" || b.Name() == "cap") {
			return true
		}
	}
	sig, ok := info.TypeOf(call.Fun).Underlying().(*types.Signature)
	if !ok {
		return false
	}
	var pt types.Type
	var pv *types.Var
	switch {
	case sig.Variadic() && idx >= sig.Params().Len()-1:
		pv = sig.Params().At(sig.Params().Len() - 1)
		pt = pv.Type().(*types.Slice).Elem()
	case idx < sig.Params().Len():
		pv = sig.Params().At(idx)
		pt = pv.Type()
	default:
		return false
	}
	if _, ok := pt.Underlying().(*types.Interface); ok {
		// Values passed together as interfaces are often compared
		// (assert.Equal): they change type together.
		for i, a := range call.Args {
			if i != idx {
				p.linkOperand(c, rw, a)
			}
		}
		return true
	}
	fn, _ := typeutil.Callee(info, call).(*types.Func)
	if fn != nil && p.prog.inLoaded(fn) {
		if fsig, ok := fn.Type().(*types.Signature); ok {
			if idx < fsig.Params().Len() {
				p.addLink(c, rw, fsig.Params().At(idx), arg)
				return true
			}
		}
	}
	return false
}

// resultVar returns the result variable a returned expression flows into.
func (p *planner) resultVar(rw *rewriter, path []ast.Node, ret *ast.ReturnStmt, e ast.Node) *types.Var {
	idx := slices.IndexFunc(ret.Results, func(r ast.Expr) bool { return r == e })
	if idx < 0 {
		return nil
	}
	info := rw.f.info()
	for _, n := range path {
		var sig *types.Signature
		switch fn := n.(type) {
		case *ast.FuncLit:
			sig, _ = info.TypeOf(fn).(*types.Signature)
		case *ast.FuncDecl:
			if obj, ok := info.Defs[fn.Name].(*types.Func); ok {
				sig = obj.Type().(*types.Signature)
			}
		default:
			continue
		}
		if sig == nil || idx >= sig.Results().Len() {
			return nil
		}
		return sig.Results().At(idx)
	}
	return nil
}
