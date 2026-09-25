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
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
	"golang.org/x/tools/go/types/typeutil"
)

// unit is a set of sites that migrate in one step: sites that share a
// legacy-typed value (a config built in one place and passed to a call in
// another) or a handler function would not compile if migrated apart.
type unit struct {
	sites []*classified
	comp  *component
	// facts are problems with uses of the unit's values outside its sites.
	facts []string
	// extra are edits of those uses (renamed fields).
	extra []intent
}

// isLegacyVar reports whether obj is a variable of the loaded code whose
// type mentions a legacy type other than a handle.
func (p *planner) isLegacyVar(obj types.Object) bool {
	v, ok := obj.(*types.Var)
	if !ok || !p.prog.inLoaded(v) || !hasLegacyType(v.Type()) {
		return false
	}
	_, handle := siblingType(v.Type())
	return !handle
}

// span is a byte range of a file.
type span struct {
	f          *srcFile
	start, end int
}

// siteSpans returns the ranges each site rewrites or covers.
func (p *planner) siteSpans(c *classified) []span {
	nodes := []ast.Node{c.s.anchor}
	for _, d := range c.s.dependents {
		nodes = append(nodes, d.anchor)
	}
	var out []span
	for _, n := range nodes {
		f := p.prog.fileOf(n.Pos())
		out = append(out, span{f, p.prog.offset(n.Pos()), p.prog.offset(n.End())})
	}
	return out
}

// buildUnits groups the classified sites into units.
func (p *planner) buildUnits() []*unit {
	var spans []span
	for _, c := range p.cls {
		spans = append(spans, p.siteSpans(c)...)
	}
	inSite := func(f *srcFile, n ast.Node) bool {
		o := p.prog.offset(n.Pos())
		for _, s := range spans {
			if s.f == f && o >= s.start && o < s.end {
				return true
			}
		}
		return false
	}
	// Variables and results a site touches.
	for _, c := range p.cls {
		info := c.s.file.info()
		for _, sp := range p.siteSpans(c) {
			path, _ := astutil.PathEnclosingInterval(sp.f.ast, p.prog.fset.File(c.s.anchor.Pos()).Pos(sp.start), p.prog.fset.File(c.s.anchor.Pos()).Pos(sp.end))
			var n ast.Node = sp.f.ast
			if len(path) > 0 {
				n = path[0]
			}
			ast.Inspect(n, func(m ast.Node) bool {
				switch x := m.(type) {
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
				}
				return true
			})
		}
	}
	// Union-find over variable keys and sites.
	parent := make(map[objKey]objKey)
	find := func(k objKey) objKey {
		if _, ok := parent[k]; !ok {
			parent[k] = k
		}
		for parent[k] != k {
			parent[k] = parent[parent[k]]
			k = parent[k]
		}
		return k
	}
	union := func(a, b objKey) { parent[find(b)] = find(a) }
	facts := make(map[objKey][]string)
	var factKeys []objKey // in source order
	var extra []struct {
		k  objKey
		it intent
	}
	// Uses of legacy-typed values outside the sites.
	for _, f := range p.prog.files {
		info := f.info()
		ast.Inspect(f.ast, func(n ast.Node) bool {
			var expr ast.Expr
			var keys []objKey
			switch x := n.(type) {
			case *ast.Ident:
				obj := info.Uses[x]
				if !p.isLegacyVar(obj) || inSite(f, x) {
					return true
				}
				keys = []objKey{p.prog.key(obj)}
				expr = x
				path, _ := astutil.PathEnclosingInterval(f.ast, x.Pos(), x.End())
				if sel, ok := parentOf(path, x).(*ast.SelectorExpr); ok && sel.Sel == x {
					expr = sel
				}
			case *ast.CallExpr:
				fn, ok := typeutil.Callee(info, x).(*types.Func)
				if !ok || !p.prog.inLoaded(fn) || inSite(f, x) {
					return true
				}
				res := fn.Type().(*types.Signature).Results()
				for i := range res.Len() {
					if p.isLegacyVar(res.At(i)) {
						keys = append(keys, p.prog.key(res.At(i)))
					}
				}
				if len(keys) == 0 {
					return true
				}
				expr = x
			default:
				return true
			}
			rw := &rewriter{prog: p.prog, f: f}
			dummy := &classified{}
			p.useContext(dummy, rw, expr)
			for _, k := range keys {
				for _, l := range dummy.links {
					union(k, l)
				}
				for _, pr := range rw.problems {
					if _, ok := facts[k]; !ok {
						factKeys = append(factKeys, k)
					}
					facts[k] = append(facts[k], pr.reason)
				}
				for _, it := range rw.extra {
					extra = append(extra, struct {
						k  objKey
						it intent
					}{k, it})
				}
			}
			return true
		})
	}
	// Sites sharing a variable group, or owned by another site, form a
	// unit.
	siteParent := make(map[*classified]*classified)
	sfind := func(c *classified) *classified {
		if _, ok := siteParent[c]; !ok {
			siteParent[c] = c
		}
		for siteParent[c] != c {
			siteParent[c] = siteParent[siteParent[c]]
			c = siteParent[c]
		}
		return c
	}
	byGroup := make(map[objKey]*classified)
	for _, c := range p.cls {
		sfind(c)
		if c.owner != nil {
			siteParent[sfind(c)] = sfind(c.owner)
		}
		for _, k := range c.links {
			g := find(k)
			if o, ok := byGroup[g]; ok {
				if a, b := sfind(o), sfind(c); a != b {
					siteParent[b] = a
				}
			} else {
				byGroup[g] = c
			}
		}
	}
	units := make(map[*classified]*unit)
	var out []*unit
	for _, c := range p.cls {
		r := sfind(c)
		u := units[r]
		if u == nil {
			u = &unit{}
			units[r] = u
			out = append(out, u)
		}
		u.sites = append(u.sites, c)
		c.unit = u
	}
	for _, g := range factKeys {
		if c, ok := byGroup[find(g)]; ok {
			u := c.unit
			u.facts = append(u.facts, facts[g]...)
		}
	}
	for _, e := range extra {
		if c, ok := byGroup[find(e.k)]; ok {
			c.unit.extra = append(c.unit.extra, e.it)
		}
	}
	for _, u := range out {
		u.facts = uniq(u.facts)
		slices.SortFunc(u.sites, func(a, b *classified) int { return p.g.cmpPos(a.s.anchor.Pos(), b.s.anchor.Pos()) })
	}
	return out
}

// useContext checks the context of a use of a legacy-typed value outside
// the sites: a field selection is checked against the jetstream type, an
// assignment's left-hand side is fine, anything else as valueContext.
func (p *planner) useContext(c *classified, rw *rewriter, e ast.Expr) {
	info := rw.f.info()
	path, _ := astutil.PathEnclosingInterval(rw.f.ast, e.Pos(), e.End())
	switch x := parentOf(path, e).(type) {
	case *ast.AssignStmt:
		if slices.Contains(x.Lhs, e) {
			if x.Tok.String() != "=" && x.Tok.String() != ":=" {
				rw.guided("%s is modified with %s", p.prog.text(e), x.Tok)
			}
			return
		}
	case *ast.ValueSpec:
		if id, ok := e.(*ast.Ident); ok && slices.Contains(x.Names, id) {
			return
		}
	case *ast.RangeStmt:
		if x.Key == e || x.Value == e {
			return
		}
		if x.X == e {
			for _, v := range []ast.Expr{x.Key, x.Value} {
				if id, ok := v.(*ast.Ident); ok {
					if obj := info.ObjectOf(id); p.isLegacyVar(obj) {
						c.links = append(c.links, p.prog.key(obj))
					}
				}
			}
			return
		}
	}
	p.valueContext(c, rw, e)
}

// fieldSelection checks a field of a legacy struct selected from a value
// whose type changes: a renamed field is rewritten, a missing one needs
// review. It reports whether the selection's own value changes type.
func (p *planner) fieldSelection(rw *rewriter, sel *ast.SelectorExpr) bool {
	info := rw.f.info()
	v, ok := info.Uses[sel.Sel].(*types.Var)
	if !ok || !v.IsField() {
		return false
	}
	recv := info.TypeOf(sel.X)
	if pt, ok := recv.(*types.Pointer); ok {
		recv = pt.Elem()
	}
	named, ok := types.Unalias(recv).(*types.Named)
	if !ok {
		return false
	}
	sym, legacy := legacyKey(named.Obj())
	if !legacy {
		return hasLegacyType(v.Type())
	}
	if nn, ok := fieldRenames[sym+"."+v.Name()]; ok {
		rw.extra = append(rw.extra, p.prog.replace(sel.Sel, nn))
		return false
	}
	if _, ok := p.prog.jsField(strings.TrimPrefix(table[sym].Target, ""), v.Name()); !ok {
		rw.guided("field %s.%s has no jetstream counterpart", sym, v.Name())
		return false
	}
	return hasLegacyType(v.Type())
}
