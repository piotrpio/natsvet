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
	"strings"
)

// bucketKey is the config field naming the bucket a create root makes.
var bucketKey = map[string]string{
	"KeyValueManager.CreateKeyValue":       "Bucket",
	"ObjectStoreManager.CreateObjectStore": "Bucket",
}

// lookupOf is the jetstream lookup a create root's sibling uses until the
// legacy root is removed.
var lookupOf = map[string]string{
	"KeyValueManager.CreateKeyValue":       "KeyValue",
	"ObjectStoreManager.CreateObjectStore": "ObjectStore",
}

// classifyRoot plans the sibling of a handle root: jetstream.New next to
// nc.JetStream, or the KeyValue or object store handle taken from the
// parent's sibling.
func (p *planner) classifyRoot(c *classified, rw *rewriter, call *ast.CallExpr) {
	s := c.s
	c.role = roleRoot
	c.ref = refInit
	var r *root
	for _, rr := range p.g.roots {
		if rr.call == call {
			r = rr
		}
	}
	handled := make(map[*ast.Ident]bool)
	markCallee(call, handled)
	if r == nil {
		// Still map the options so a legacy-only one is reported.
		if s.sym == "Conn.JetStream" {
			for _, o := range call.Args {
				p.mapOption(rw, o, false, nil, handled)
			}
		}
		rw.guided("the handle is not assigned to a variable next to its error; assign it (js, err := ...) so a sibling can be created")
		c.summary = fmt.Sprintf("%s creates a handle the planner cannot thread", s.sym)
		p.settle(c, rw, nil)
		return
	}
	h := p.g.handles[r.to]
	if r.why != "" {
		rw.guided("%s", r.why)
	}
	if h != nil && !h.threadable {
		rw.guided("the handle cannot be threaded: %s", h.why)
	}
	if h == nil {
		rw.guided("the handle is not a variable or field the planner threads")
	}
	f := s.file
	info := f.info()
	b := &builder{}
	var create *builder
	sel, _ := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	switch s.sym {
	case "Conn.JetStream":
		c.summary = "nc.JetStream becomes jetstream.New"
		if sel == nil || !isPlainRef(sel.X) {
			rw.guided("the connection is an expression the sibling would evaluate again; assign it to a variable first")
		}
		if call.Ellipsis.IsValid() {
			rw.guided("handle options are built at runtime: map each to its jetstream.JetStreamOpt, and nats.Domain and nats.APIPrefix to the NewWithDomain and NewWithAPIPrefix constructors")
		}
		ctor, ctorArg := "New", ""
		var opts []string
		var elem types.Type
		if o := p.prog.jsObject("JetStreamOpt"); o != nil {
			elem = o.Type()
		}
		for _, o := range call.Args {
			oc, ok := ast.Unparen(o).(*ast.CallExpr)
			sym := ""
			if ok {
				sym, _ = calleeKey(info, oc)
			}
			switch sym {
			case "Domain", "APIPrefix":
				markCallee(oc, handled)
				if ctor != "New" || len(oc.Args) != 1 {
					rw.guided("both nats.Domain and nats.APIPrefix are set; choose the jetstream constructor by hand")
					continue
				}
				ctor, ctorArg = table[sym].Target, p.exprText(rw, oc.Args[0], handled)
			case "Context", "ContextOpt":
				markCallee(oc, handled)
				rw.guided("a handle-level nats.Context has no jetstream counterpart; pass the context to each call")
			case "PublishAsyncErrHandler":
				markCallee(oc, handled)
				rw.guided("the error handler's first parameter becomes jetstream.JetStream: pass jetstream.WithPublishAsyncErrHandler a func(jetstream.JetStream, *nats.Msg, error)")
			default:
				if r := p.mapOption(rw, o, false, elem, handled); r.text != "" {
					opts = append(opts, r.text)
				}
			}
		}
		args := []string{}
		if sel != nil {
			args = append(args, p.prog.text(sel.X))
		}
		if ctorArg != "" {
			args = append(args, ctorArg)
		}
		args = append(args, opts...)
		p.rootStmt(b, r, h, "jetstream."+ctor+"("+strings.Join(args, ", ")+")")
	default:
		parent := p.g.handles[r.recv]
		if parent == nil || sel == nil {
			rw.guided("the handle is derived from an expression the planner does not thread")
			break
		}
		if !parent.threadable {
			rw.guided("the parent handle cannot be threaded: %s", parent.why)
		}
		target := table[s.sym].Target
		_, member, _ := strings.Cut(target, ".")
		ctx, _ := p.prog.ctxAt(f, call.Pos(), nil)
		c.summary = fmt.Sprintf("%s becomes %s", s.sym, target)
		recv := &builder{}
		p.siblingRecv(recv, sel.X, parent)
		if key, ok := bucketKey[s.sym]; ok {
			// A create root: the sibling looks the bucket up until the
			// legacy root, which creates it, is removed.
			lit := configLit(call)
			bucket := litField(lit, key)
			if lit == nil || bucket == nil {
				rw.guided("the bucket config is not an inline literal with a %s; create the sibling by hand", key)
				break
			}
			cfg := p.prog.applyWithin(lit, rw.litIntents(lit, s.uses))
			for _, u := range s.uses {
				if u.id.Pos() >= lit.Pos() && u.id.End() <= lit.End() {
					handled[u.id] = true
				}
			}
			get := &builder{}
			get.embed(recv.String(), recv.marks).add(".", lookupOf[s.sym], "(", ctx, ", ", p.prog.text(bucket), ")")
			p.rootStmt(b, r, h, "")
			b.embed(get.String(), get.marks)
			create = &builder{}
			p.rootLHS(create, r, h)
			create.embed(recv.String(), recv.marks).add(".", member, "(", ctx, ", ", cfg, ")")
			break
		}
		args := []string{ctx}
		for _, a := range call.Args {
			args = append(args, p.exprText(rw, a, handled))
		}
		get := &builder{}
		get.embed(recv.String(), recv.marks).add(".", member, "(", strings.Join(args, ", "), ")")
		p.rootStmt(b, r, h, "")
		b.embed(get.String(), get.marks)
	}
	if len(r.stmt.Lhs) == 2 {
		if id, ok := r.stmt.Lhs[1].(*ast.Ident); ok && id.Name != "_" && r.errCheck == nil {
			rw.guided("no `if err != nil` check directly follows the root; the sibling needs the same error handling")
		}
	}
	p.checkHandled(rw, s, handled, nil)
	p.settle(c, rw, nil)
	if c.class != classMechanical || h == nil {
		return
	}
	rp := &rootPlan{r: r}
	indent := p.prog.indentOf(r.stmt.Pos())
	text := &builder{}
	text.add(indent).embed(b.String(), b.marks).add("\n")
	if r.errCheck != nil {
		text.add(indent, p.prog.text(r.errCheck), "\n")
	}
	rp.text, rp.marks = text.String(), text.marks
	if create != nil {
		rp.create, rp.createMarks = create.String(), create.marks
	}
	c.root = rp
	var lines []string
	for _, l := range strings.Split(strings.TrimRight(rp.text, "\n"), "\n") {
		lines = append(lines, strings.TrimPrefix(l, indent))
	}
	c.after = p.prog.text(r.stmt) + "\n" + strings.Join(lines, "\n")
}

// rootLHS writes a root's left-hand side with the handle replaced by its
// sibling, and the assignment operator.
func (p *planner) rootLHS(b *builder, r *root, h *handle) {
	p.siblingRecv(b, r.stmt.Lhs[0], h)
	for _, l := range r.stmt.Lhs[1:] {
		b.add(", ", p.prog.text(l))
	}
	b.add(" ", r.stmt.Tok.String(), " ")
}

// rootStmt writes the sibling root statement: the LHS, then rhs.
func (p *planner) rootStmt(b *builder, r *root, h *handle, rhs string) {
	p.rootLHS(b, r, h)
	b.add(rhs)
}

// isPlainRef reports whether e is an identifier or a selector chain of
// identifiers, safe to evaluate twice.
func isPlainRef(e ast.Expr) bool {
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return isPlainRef(x.X)
	}
	return false
}

// configLit returns the composite literal a create call's first argument
// points to.
func configLit(call *ast.CallExpr) *ast.CompositeLit {
	if len(call.Args) == 0 {
		return nil
	}
	u, ok := ast.Unparen(call.Args[0]).(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return nil
	}
	lit, _ := ast.Unparen(u.X).(*ast.CompositeLit)
	return lit
}

// litField returns the value of a keyed field of a literal.
func litField(lit *ast.CompositeLit, key string) ast.Expr {
	if lit == nil {
		return nil
	}
	for _, e := range lit.Elts {
		if kv, ok := e.(*ast.KeyValueExpr); ok {
			if id, ok := kv.Key.(*ast.Ident); ok && id.Name == key {
				return kv.Value
			}
		}
	}
	return nil
}
