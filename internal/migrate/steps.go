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
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/format"
	"slices"
	"strings"

	"golang.org/x/tools/go/ast/astutil"
)

// Step kinds.
const (
	stepGoGet     = "go-get"
	stepAdd       = "add-handle"
	stepSite      = "site"
	stepFinish    = "finish"
	stepComponent = "component"
)

// stepPlan is a step before simulation.
type stepPlan struct {
	kind    string
	comp    *component
	summary string
	sites   []*classified
	// intents are edits in original coordinates.
	intents []intent
	// current computes edits in the coordinates of the simulated files at
	// the time the step runs.
	current func(sm *sim) ([]intent, error)
	// machine: the step carries edits an applier can apply.
	machine bool
	waitsOn []string
	facts   []string
	command string
	// parts are the steps a component step runs as one.
	parts []*stepPlan
	out   *Step
}

// compPlan is the steps of one component.
type compPlan struct {
	comp      *component
	add       *stepPlan
	units     []*stepPlan
	finish    *stepPlan
	blocked   []boundary
	skipped   bool
	roots     []*classified
	decls     []*classified
	blockers  []string // site ids that keep the legacy handle
	oneCommit bool
}

// assemble orders the steps of every component and of the units outside
// components.
func (p *planner) assemble(comps []*component, units []*unit) []*stepPlan {
	var steps []*stepPlan
	if st := stepZero(p.prog.natsVersion); st != nil {
		steps = append(steps, st)
	}
	byComp := make(map[*component][]*unit)
	var loose []*unit
	for _, u := range units {
		if u.comp != nil {
			byComp[u.comp] = append(byComp[u.comp], u)
		} else {
			loose = append(loose, u)
		}
	}
	for _, comp := range comps {
		cp := p.planComponent(comp, byComp[comp])
		if cp.skipped {
			continue
		}
		parts := append([]*stepPlan{cp.add}, cp.units...)
		parts = append(parts, cp.finish)
		parts = slices.DeleteFunc(parts, func(st *stepPlan) bool { return st == nil })
		if cp.oneCommit && !slices.ContainsFunc(parts, func(st *stepPlan) bool { return !st.machine }) {
			steps = append(steps, p.componentStep(comp, parts))
			continue
		}
		steps = append(steps, parts...)
	}
	for _, u := range orderUnits(loose) {
		steps = append(steps, p.unitStep(nil, u, nil))
	}
	var out []*stepPlan
	for _, st := range steps {
		if st != nil {
			out = append(out, st)
		}
	}
	return out
}

// stepZero raises a module's nats.go to the version the table was verified
// against, when it requires an older one.
func stepZero(module string) *stepPlan {
	if !older(module, tableVersion) {
		return nil
	}
	return &stepPlan{kind: stepGoGet, command: "go get " + natsModule + "@" + tableVersion,
		summary: fmt.Sprintf("raise nats.go from %s to %s, the version the mapping and its behavior facts were verified against; later v1 releases only add API", module, tableVersion)}
}

// versionNote tells how the module's nats.go relates to the table's.
func versionNote(module string) string {
	switch {
	case older(module, tableVersion):
		return fmt.Sprintf("the module requires nats.go %s: apply step S0 and plan again; this plan was classified against the jetstream package of %s", module, module)
	case older(tableVersion, module):
		return fmt.Sprintf("the module requires nats.go %s; the behavior facts in this plan were verified against %s", module, tableVersion)
	}
	return ""
}

// componentStep makes the one step of a component safe in one commit: its
// parts run in order, and the plan shows their combined edits.
func (p *planner) componentStep(comp *component, parts []*stepPlan) *stepPlan {
	st := &stepPlan{kind: stepComponent, comp: comp, machine: true, parts: parts}
	var names, summaries []string
	for _, h := range comp.handles {
		names = append(names, h.ident.Name)
	}
	for _, part := range parts {
		for _, c := range part.sites {
			if !slices.Contains(st.sites, c) {
				st.sites = append(st.sites, c)
			}
		}
		if part.kind == stepSite && part.summary != "" {
			summaries = append(summaries, part.summary)
		}
	}
	st.summary = "migrate " + strings.Join(names, ", ") + " to the jetstream package in one step"
	if len(summaries) > 0 {
		st.summary += ": " + strings.Join(uniq(summaries), "; ")
	}
	return st
}

// orderUnits puts units whose sites are all mechanical first.
func orderUnits(us []*unit) []*unit {
	rank := func(u *unit) int {
		r := 0
		for _, c := range u.sites {
			switch c.class {
			case classGuided:
				r = max(r, 1)
			case classDecision:
				r = max(r, 2)
			case classUnmapped:
				r = max(r, 3)
			}
		}
		if len(u.facts) > 0 {
			r = max(r, 1)
		}
		return r
	}
	out := slices.Clone(us)
	slices.SortStableFunc(out, func(a, b *unit) int { return rank(a) - rank(b) })
	return out
}

// unitStep makes the step of one unit. A unit whose sites are not all
// mechanical, or whose component cannot get its siblings, carries no
// edits.
func (p *planner) unitStep(comp *component, u *unit, addBlockers []string) *stepPlan {
	// Unmapped sites have no step: no step can migrate them.
	listed := slices.DeleteFunc(slices.Clone(u.sites), func(c *classified) bool { return c.class == classUnmapped })
	if len(listed) == 0 {
		return nil
	}
	st := &stepPlan{kind: stepSite, comp: comp, sites: listed, machine: true}
	var parts []string
	var intents []intent
	for _, c := range u.sites {
		if (c.role != roleSite && comp != nil) || c.owner != nil {
			continue
		}
		if c.summary != "" {
			parts = append(parts, c.summary)
		}
		if c.class != classMechanical {
			st.machine = false
			continue
		}
		intents = append(intents, c.intents...)
	}
	intents = append(intents, u.extra...)
	st.facts = append(st.facts, u.facts...)
	if len(u.facts) > 0 {
		st.machine = false
	}
	if len(addBlockers) > 0 {
		st.machine = false
		st.waitsOn = addBlockers
	}
	st.summary = strings.Join(uniq(parts), "; ")
	if st.machine {
		deduped, overlap := dedupeIntents(intents)
		if overlap {
			st.machine = false
			st.facts = append(st.facts, "the planner's edits for these sites overlap; migrate them by hand")
		}
		st.intents = deduped
	}
	return st
}

// dedupeIntents drops identical intents and reports whether the rest
// overlap.
func dedupeIntents(in []intent) ([]intent, bool) {
	sorted, _ := sortIntents(slices.Clone(in))
	var out []intent
	for _, it := range sorted {
		if n := len(out); n > 0 {
			last := out[n-1]
			if last.file == it.file && last.start == it.start && last.end == it.end && last.text == it.text {
				continue
			}
		}
		out = append(out, it)
	}
	_, overlap := sortIntents(out)
	return out, overlap
}

// planComponent makes the add-handle, site, removal and rename steps of a
// component.
func (p *planner) planComponent(comp *component, units []*unit) *compPlan {
	cp := &compPlan{comp: comp, skipped: comp.skipped, blocked: comp.boundaries}
	if cp.skipped {
		return cp
	}
	for _, u := range units {
		for _, c := range u.sites {
			switch c.role {
			case roleRoot:
				cp.roots = append(cp.roots, c)
			case roleDecl:
				cp.decls = append(cp.decls, c)
			}
		}
	}
	// Add-handle.
	add := &stepPlan{kind: stepAdd, comp: comp, machine: true}
	var addBlockers []string
	// Roots and declarations of handles that get a sibling must be
	// mechanical; those of handles that cannot keep the legacy handle
	// alive instead, holding back only removal and rename.
	var holdBlockers []string
	for _, c := range append(slices.Clone(cp.roots), cp.decls...) {
		add.sites = append(add.sites, c)
		if c.class == classMechanical {
			continue
		}
		if h := p.handleOfSite(c); h != nil && h.threadable {
			add.machine = false
			addBlockers = append(addBlockers, c.id)
		} else {
			holdBlockers = append(holdBlockers, c.id)
		}
	}
	threadable := false
	var whys []string
	for _, h := range comp.handles {
		if h.threadable {
			threadable = true
		} else {
			whys = append(whys, fmt.Sprintf("%s at %s cannot get a sibling: %s", h.ident.Name, p.posID(h.ident.Pos()), h.why))
		}
	}
	if !threadable {
		add.machine = false
		add.facts = append(add.facts, whys...)
	}
	// Siblings an earlier add-handle step declared: when every threadable
	// handle has one, the step is done, and a root that was guided holds
	// back only the finish step.
	var present, missing []string
	for _, h := range comp.handles {
		switch {
		case !h.threadable:
		case h.present != nil:
			present = append(present, h.sibling)
		default:
			missing = append(missing, h.ident.Name)
		}
	}
	resumed := len(present) > 0 && len(missing) == 0
	if len(present) > 0 && len(missing) > 0 {
		add.machine = false
		add.facts = append(add.facts, fmt.Sprintf("the siblings %s are declared but %s have none; declare the missing ones as this step would, or remove the declared ones, and plan again", strings.Join(present, ", "), strings.Join(missing, ", ")))
	}
	switch {
	case resumed:
		holdBlockers = append(holdBlockers, addBlockers...)
		addBlockers = nil
	case add.machine:
		intents, facts := p.addIntents(comp, cp)
		if len(facts) > 0 {
			add.machine = false
			add.facts = facts
		} else {
			add.intents = intents
		}
	}
	if !resumed && !add.machine && len(addBlockers) == 0 {
		addBlockers = append(addBlockers, "add-handle")
	}
	var names []string
	for _, h := range comp.handles {
		if h.threadable {
			names = append(names, h.sibling)
		}
	}
	add.summary = "create the jetstream siblings " + strings.Join(names, ", ") + " next to the legacy handles"
	if len(comp.handles) == 0 || resumed {
		add = nil
	}
	cp.add = add
	blocking := addBlockers
	if add != nil && add.machine {
		blocking = nil
	}
	// Units: mechanical first.
	var handleUnits []*unit
	for _, u := range units {
		own := false
		for _, c := range u.sites {
			if c.role == roleSite {
				own = true
			}
		}
		if own {
			handleUnits = append(handleUnits, u)
		}
	}
	for _, u := range orderUnits(handleUnits) {
		st := p.unitStep(comp, u, blocking)
		if st == nil || !st.machine {
			for _, c := range u.sites {
				if c.role == roleSite && c.class != classMechanical {
					cp.blockers = append(cp.blockers, c.id)
				}
			}
		}
		if st == nil {
			continue
		}
		cp.units = append(cp.units, st)
		if !st.machine && (len(u.facts) > 0 || len(st.facts) > 0) {
			cp.blockers = append(cp.blockers, u.sites[0].id)
		}
	}
	cp.blockers = uniq(append(append(slices.Clone(blocking), holdBlockers...), cp.blockers...))
	if len(cp.blocked) > 0 {
		return cp
	}
	// Removal and rename, in one step.
	fin := &stepPlan{kind: stepFinish, comp: comp, summary: "remove the legacy handles, their roots and the values threaded into them, and rename each sibling to its legacy name: " + renames(comp), sites: append(slices.Clone(cp.roots), cp.decls...)}
	if len(cp.blockers) > 0 {
		fin.waitsOn = cp.blockers
	} else {
		fin.machine = true
		fin.current = func(sm *sim) ([]intent, error) { return p.finishIntents(sm, comp, cp) }
	}
	cp.finish = fin
	cp.oneCommit = len(cp.blockers) == 0 && p.onePackage(comp, units)
	comp.oneCommit = cp.oneCommit
	return cp
}

// handleOfSite returns the handle a root or declaration site creates.
func (p *planner) handleOfSite(c *classified) *handle {
	if k, ok := p.g.siteHandle(c.s); ok {
		return p.g.handles[k]
	}
	return nil
}

func renames(comp *component) string {
	var out []string
	for _, h := range comp.handles {
		out = append(out, h.sibling+" → "+h.ident.Name)
	}
	return strings.Join(out, ", ")
}

// onePackage reports whether a component's sites and handles are all in
// one package.
func (p *planner) onePackage(comp *component, units []*unit) bool {
	pkgs := make(map[string]bool)
	for _, h := range comp.handles {
		pkgs[h.file.pkg.PkgPath] = true
	}
	for _, u := range units {
		for _, c := range u.sites {
			pkgs[c.s.file.pkg.PkgPath] = true
		}
	}
	return len(pkgs) == 1
}

// addIntents builds the add-handle edits: sibling declarations, sibling
// roots, and the values threaded into siblings next to the legacy flows.
func (p *planner) addIntents(comp *component, cp *compPlan) ([]intent, []string) {
	var out []intent
	var facts []string
	byKey := make(map[objKey]*handle)
	for _, h := range comp.handles {
		byKey[h.key] = h
	}
	// Siblings read after this step, so that a local sibling is never
	// unused.
	read := make(map[objKey]bool)
	for _, fl := range comp.flows {
		if from, to := byKey[fl.from], byKey[fl.to]; from != nil && to != nil && from.threadable && to.threadable {
			read[fl.from] = true
		}
	}
	for _, c := range cp.roots {
		if c.class == classMechanical && c.root != nil && c.root.r.recv != "" {
			read[c.root.r.recv] = true
		}
	}
	for _, c := range cp.decls {
		if c.class != classMechanical {
			continue
		}
		it, err := p.declIntent(c)
		if err != "" {
			facts = append(facts, err)
			continue
		}
		out = append(out, it)
	}
	for _, c := range cp.roots {
		rp := c.root
		if c.class != classMechanical || rp == nil {
			continue
		}
		r := rp.r
		h := byKey[r.to]
		end := r.stmt.End()
		if r.errCheck != nil {
			end = r.errCheck.End()
		}
		_, at := p.prog.lineSpan(r.stmt.Pos(), end)
		b := &builder{}
		start := b.len()
		b.embed(rp.text, rp.marks)
		b.span(start, "root:"+c.id)
		if h != nil && h.kind == handleLocal {
			p.placeholders(b, h, p.prog.indentOf(r.stmt.Pos()), read[h.key])
		}
		out = append(out, b.at(c.s.file, at, at))
	}
	for _, fl := range comp.flows {
		from, to := byKey[fl.from], byKey[fl.to]
		if from == nil || to == nil || !from.threadable || !to.threadable {
			continue
		}
		it, err := p.flowIntent(fl, from, to, read)
		if err != "" {
			facts = append(facts, err)
			continue
		}
		out = append(out, it)
	}
	if _, overlap := sortIntents(slices.Clone(out)); overlap {
		facts = append(facts, "the sibling declarations overlap; add them by hand")
	}
	return out, facts
}

// placeholders keeps a local legacy handle, and a local sibling nothing
// reads yet, used until the removal step deletes these lines: every site
// that reads the legacy handle may migrate before then.
func (p *planner) placeholders(b *builder, h *handle, indent string, siblingRead bool) {
	ps := b.len()
	b.add(indent, "_ = ", h.ident.Name, "\n")
	if !siblingRead {
		b.add(indent, "_ = ").sib(h).add("\n")
	}
	b.span(ps, "placeholder:"+string(h.key))
}

// declIntent declares a handle's sibling next to its declaration.
func (p *planner) declIntent(c *classified) (intent, string) {
	info := c.s.file.info()
	names, _ := declNames(c.s.anchor)
	if len(names) != 1 {
		return intent{}, c.id + ": the handle is declared together with other names"
	}
	h := p.g.handles[p.prog.key(info.Defs[names[0]])]
	if h == nil {
		return intent{}, c.id + ": the declaration is not a threaded handle"
	}
	b := &builder{}
	switch d := c.s.anchor.(type) {
	case *ast.Field:
		if h.kind == handleField {
			_, at := p.prog.lineSpan(d.Pos(), d.End())
			b.add(p.prog.indentOf(d.Pos())).sib(h).add(" ", h.newType, "\n")
			return b.at(c.s.file, at, at), ""
		}
		b.add(", ").sib(h).add(" ", h.newType)
		o := p.prog.offset(d.End())
		return b.at(c.s.file, o, o), ""
	case *ast.ValueSpec:
		if len(d.Values) > 0 {
			return intent{}, c.id + ": the handle is declared with a value; declare its sibling by hand"
		}
		grouped := false
		for i := len(c.s.stack) - 1; i >= 0; i-- {
			if gd, ok := c.s.stack[i].(*ast.GenDecl); ok {
				grouped = gd.Lparen.IsValid()
				break
			}
		}
		_, at := p.prog.lineSpan(d.Pos(), d.End())
		b.add(p.prog.indentOf(d.Pos()))
		if !grouped {
			b.add("var ")
		}
		b.sib(h).add(" ", h.newType, "\n")
		return b.at(c.s.file, at, at), ""
	}
	return intent{}, c.id + ": unsupported handle declaration"
}

// flowIntent threads a sibling value next to a legacy flow.
func (p *planner) flowIntent(fl *flow, from, to *handle, read map[objKey]bool) (intent, string) {
	f := fl.file
	where := p.posID(fl.node.Pos())
	b := &builder{}
	sibOf := func(e ast.Expr, h *handle) {
		p.siblingRecv(b, e, h)
	}
	switch fl.kind {
	case flowAssign:
		switch n := fl.node.(type) {
		case *ast.AssignStmt:
			if !p.inBlock(f, n) {
				return intent{}, where + ": the handle is assigned inside a statement header; thread its sibling by hand"
			}
			_, at := p.prog.lineSpan(n.Pos(), n.End())
			b.add(p.prog.indentOf(n.Pos()))
			sibOf(fl.toExpr, to)
			b.add(" ", n.Tok.String(), " ")
			sibOf(fl.fromExpr, from)
			b.add("\n")
			if to.kind == handleLocal && fl.define {
				p.placeholders(b, to, p.prog.indentOf(n.Pos()), read[to.key])
			}
			return b.at(f, at, at), ""
		case *ast.ValueSpec:
			if len(n.Names) != 1 || n.Type != nil {
				return intent{}, where + ": the handle is declared with a value; declare its sibling by hand"
			}
			_, at := p.prog.lineSpan(n.Pos(), n.End())
			b.add(p.prog.indentOf(n.Pos()), "var ")
			sibOf(fl.toExpr, to)
			b.add(" = ")
			sibOf(fl.fromExpr, from)
			b.add("\n")
			return b.at(f, at, at), ""
		}
	case flowLitField:
		kv := fl.node.(*ast.KeyValueExpr)
		b.add(", ")
		b.sib(to)
		b.add(": ")
		sibOf(fl.fromExpr, from)
		o := p.prog.offset(kv.End())
		return b.at(f, o, o), ""
	case flowArg:
		b.add(", ")
		sibOf(fl.fromExpr, from)
		o := p.prog.offset(fl.fromExpr.End())
		return b.at(f, o, o), ""
	}
	return intent{}, where + ": the handle flows in a way the planner does not thread"
}

// inBlock reports whether a statement is directly in a block.
func (p *planner) inBlock(f *srcFile, st ast.Stmt) bool {
	found := false
	ast.Inspect(f.ast, func(n ast.Node) bool {
		if found {
			return false
		}
		switch b := n.(type) {
		case *ast.BlockStmt:
			if slices.Contains(b.List, st) {
				found = true
			}
		case *ast.CaseClause:
			if slices.Contains(b.Body, st) {
				found = true
			}
		case *ast.CommClause:
			if slices.Contains(b.Body, st) {
				found = true
			}
		}
		return true
	})
	return found
}

// removeIntents deletes the legacy declarations, roots and flows of a
// component, in current coordinates.
func (p *planner) removeIntents(sm *sim, comp *component, cp *compPlan) ([]intent, error) {
	var out []intent
	del := func(f *srcFile, start, end int) error {
		s, err := sm.cur(f, start, true)
		if err != nil {
			return err
		}
		e, err := sm.cur(f, end, false)
		if err != nil {
			return err
		}
		out = append(out, intent{file: f, start: s, end: e})
		return nil
	}
	// delPlus deletes [start, end) and the ", " inserted right after it.
	delPlus := func(f *srcFile, start, end int) error {
		s, err := sm.cur(f, start, true)
		if err != nil {
			return err
		}
		e, err := sm.cur(f, end, false)
		if err != nil {
			return err
		}
		out = append(out, intent{file: f, start: s, end: e + 2})
		return nil
	}
	for _, c := range cp.decls {
		if c.class != classMechanical {
			continue
		}
		switch d := c.s.anchor.(type) {
		case *ast.Field:
			if p.g.handles[p.prog.key(c.s.file.info().Defs[d.Names[0]])].kind == handleField {
				s, e := p.prog.lineSpan(d.Pos(), d.End())
				if err := del(c.s.file, s, e); err != nil {
					return nil, err
				}
				continue
			}
			if err := delPlus(c.s.file, p.prog.offset(d.Pos()), p.prog.offset(d.End())); err != nil {
				return nil, err
			}
		case *ast.ValueSpec:
			var n ast.Node = d
			for i := len(c.s.stack) - 1; i >= 0; i-- {
				if gd, ok := c.s.stack[i].(*ast.GenDecl); ok && !gd.Lparen.IsValid() {
					n = gd
					break
				}
			}
			s, e := p.prog.lineSpan(n.Pos(), n.End())
			if err := del(c.s.file, s, e); err != nil {
				return nil, err
			}
		}
	}
	for _, c := range cp.roots {
		rp := c.root
		if c.class != classMechanical || rp == nil {
			continue
		}
		r := rp.r
		f := c.s.file
		if rp.create != "" {
			// The legacy root becomes the jetstream create call; the
			// sibling lookup goes.
			s, err := sm.cur(f, p.prog.offset(r.stmt.Pos()), true)
			if err != nil {
				return nil, err
			}
			e, err := sm.cur(f, p.prog.offset(r.stmt.End()), false)
			if err != nil {
				return nil, err
			}
			out = append(out, intent{file: f, start: s, end: e, text: rp.create, marks: rp.createMarks})
		} else {
			end := r.stmt.End()
			if r.errCheck != nil {
				end = r.errCheck.End()
			}
			s, e := p.prog.lineSpan(r.stmt.Pos(), end)
			if err := del(f, s, e); err != nil {
				return nil, err
			}
		}
		if rp.create != "" {
			if sp, ok := sm.span(f, "root:"+c.id); ok {
				out = append(out, intent{file: f, start: sp[0], end: sp[1]})
			}
		}
	}
	for _, h := range comp.handles {
		for _, sp := range sm.spans(h.file, "placeholder:"+string(h.key)) {
			out = append(out, intent{file: h.file, start: sp[0], end: sp[1]})
		}
	}
	byKey := make(map[objKey]*handle)
	for _, h := range comp.handles {
		byKey[h.key] = h
	}
	for _, fl := range comp.flows {
		if from, to := byKey[fl.from], byKey[fl.to]; from == nil || to == nil || !from.threadable || !to.threadable {
			continue
		}
		f := fl.file
		switch fl.kind {
		case flowAssign:
			s, e := p.prog.lineSpan(fl.node.Pos(), fl.node.End())
			if err := del(f, s, e); err != nil {
				return nil, err
			}
		case flowLitField:
			kv := fl.node.(*ast.KeyValueExpr)
			if err := delPlus(f, p.prog.offset(kv.Pos()), p.prog.offset(kv.End())); err != nil {
				return nil, err
			}
		case flowArg:
			if err := delPlus(f, p.prog.offset(fl.fromExpr.Pos()), p.prog.offset(fl.fromExpr.End())); err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

// finishIntents removes a component's legacy handles and renames its
// siblings, leaving out the renames inside removed text.
func (p *planner) finishIntents(sm *sim, comp *component, cp *compPlan) ([]intent, error) {
	rm, err := p.removeIntents(sm, comp, cp)
	if err != nil {
		return nil, err
	}
	rn, err := p.renameIntents(sm, comp)
	if err != nil {
		return nil, err
	}
	keys := make(map[string]bool)
	for _, h := range comp.handles {
		keys[string(h.key)] = true
	}
	out := make([]intent, 0, len(rm)+len(rn))
	for _, it := range rm {
		out = append(out, renameInText(it, keys))
	}
	for _, it := range rn {
		if !slices.ContainsFunc(rm, func(r intent) bool {
			return r.file == it.file && r.start < r.end && r.start <= it.start && it.end <= r.end
		}) {
			out = append(out, it)
		}
	}
	return out, nil
}

// renameInText renames the siblings of keys inside the text an intent
// inserts, and moves the intent's other marks accordingly.
func renameInText(it intent, keys map[string]bool) intent {
	type rename struct{ off, n, delta int }
	var renames []rename
	var b strings.Builder
	last := 0
	marks := slices.Clone(it.marks)
	slices.SortFunc(marks, func(a, b mark) int { return a.off - b.off })
	for _, m := range marks {
		if m.kind != markSibling || !keys[m.key] {
			continue
		}
		b.WriteString(it.text[last:m.off])
		b.WriteString(m.orig)
		last = m.off + m.n
		renames = append(renames, rename{m.off, m.n, len(m.orig) - m.n})
	}
	if len(renames) == 0 {
		return it
	}
	b.WriteString(it.text[last:])
	shift := func(o int) int {
		d := 0
		for _, r := range renames {
			if r.off+r.n <= o {
				d += r.delta
			}
		}
		return o + d
	}
	var kept []mark
	for _, m := range marks {
		if m.kind == markSibling && keys[m.key] {
			continue
		}
		end := shift(m.off + m.n)
		m.off = shift(m.off)
		m.n = end - m.off
		kept = append(kept, m)
	}
	it.text, it.marks = b.String(), kept
	return it
}

// renameIntents renames every sibling of a component to its legacy name.
func (p *planner) renameIntents(sm *sim, comp *component) ([]intent, error) {
	keys := make(map[string]bool)
	for _, h := range comp.handles {
		keys[string(h.key)] = true
	}
	var out []intent
	for _, f := range sm.order {
		sf := sm.files[f]
		for _, m := range sf.marks {
			if m.kind == markSibling && keys[m.key] {
				out = append(out, intent{file: f, start: m.off, end: m.off + m.n, text: m.orig})
			}
		}
	}
	return out, nil
}

// sim is the simulated state of the module's files while steps apply.
type sim struct {
	p     *planner
	files map[*srcFile]*simFile
	order []*srcFile
}

type simFile struct {
	text  []byte
	log   [][]logEdit
	marks []mark // offsets in the current text
}

// logEdit is an applied edit in the coordinates before its step.
type logEdit struct{ start, end, n int }

func newSim(p *planner) *sim {
	sm := &sim{p: p, files: make(map[*srcFile]*simFile)}
	for _, f := range p.prog.files {
		sm.files[f] = &simFile{text: f.src}
		sm.order = append(sm.order, f)
	}
	return sm
}

// cur maps an original offset of f to the current text. Text inserted
// exactly at the offset counts as before it when after is set.
func (sm *sim) cur(f *srcFile, o int, after bool) (int, error) {
	for _, batch := range sm.files[f].log {
		delta := 0
		for _, e := range batch {
			switch {
			case e.start == e.end:
				if e.start < o || (e.start == o && after) {
					delta += e.n
				}
			case e.end <= o:
				delta += e.n - (e.end - e.start)
			case e.start < o:
				return 0, fmt.Errorf("%s: offset %d lies in text an earlier step rewrote", f.rel, o)
			}
		}
		o += delta
	}
	return o, nil
}

// seed loads the siblings, placeholders and sibling roots an earlier
// add-handle step left in the loaded code into the marks, as that step
// leaves them in the simulation, so that the finish step finds them.
func (sm *sim) seed() {
	p := sm.p
	for _, h := range p.g.handles {
		if h.present == nil {
			continue
		}
		sk := p.prog.key(h.file.info().Defs[h.present])
		legacy := p.prog.key(h.file.info().Defs[h.ident])
		for _, f := range sm.order {
			info, sf := f.info(), sm.files[f]
			ast.Inspect(f.ast, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && p.prog.key(info.ObjectOf(id)) == sk {
					sf.marks = append(sf.marks, mark{off: p.prog.offset(id.Pos()), n: len(id.Name), kind: markSibling, name: h.sibling, orig: h.ident.Name, key: string(h.key)})
				}
				return true
			})
		}
		info, sf := h.file.info(), sm.files[h.file]
		ast.Inspect(h.file.ast, func(n ast.Node) bool {
			if x, ok := isPlaceholder(n); ok {
				if k := p.prog.key(info.ObjectOf(ast.Unparen(x).(*ast.Ident))); k == legacy || k == sk {
					s, e := p.prog.lineSpan(n.Pos(), n.End())
					sf.marks = append(sf.marks, mark{off: s, n: e - s, kind: markSpan, key: "placeholder:" + string(h.key)})
				}
			}
			return true
		})
	}
	for _, c := range p.cls {
		if c.role != roleRoot || c.root == nil || c.root.create == "" {
			continue
		}
		h := p.g.handles[c.root.r.to]
		if h == nil || h.present == nil {
			continue
		}
		path, _ := astutil.PathEnclosingInterval(h.file.ast, h.present.Pos(), h.present.End())
		for i, n := range path {
			as, ok := n.(*ast.AssignStmt)
			if !ok || i+1 >= len(path) {
				continue
			}
			end := as.End()
			if ec := followingErrCheck([]ast.Node{path[i+1], as}, as); ec != nil {
				end = ec.End()
			}
			s, e := p.prog.lineSpan(as.Pos(), end)
			sf := sm.files[h.file]
			sf.marks = append(sf.marks, mark{off: s, n: e - s, kind: markSpan, key: "root:" + c.id})
			break
		}
	}
	for _, sf := range sm.files {
		slices.SortFunc(sf.marks, func(a, b mark) int { return a.off - b.off })
	}
}

// spans returns the current ranges of the span marks with key.
func (sm *sim) spans(f *srcFile, key string) [][2]int {
	var out [][2]int
	for _, m := range sm.files[f].marks {
		if m.kind == markSpan && m.key == key {
			out = append(out, [2]int{m.off, m.off + m.n})
		}
	}
	return out
}

// span returns the current range of a span mark.
func (sm *sim) span(f *srcFile, key string) ([2]int, bool) {
	for _, m := range sm.files[f].marks {
		if m.kind == markSpan && m.key == key {
			return [2]int{m.off, m.off + m.n}, true
		}
	}
	return [2]int{}, false
}

// simState is a file's place in the simulation: its log length and text.
type simState struct {
	n    int
	text []byte
}

// state records every file's place, to compose the batches logged after
// it.
func (sm *sim) state() map[*srcFile]simState {
	out := make(map[*srcFile]simState, len(sm.files))
	for f, sf := range sm.files {
		out[f] = simState{n: len(sf.log), text: sf.text}
	}
	return out
}

// snapshot copies the simulated files, to restore them when a component
// step fails.
func (sm *sim) snapshot() map[*srcFile]simFile {
	out := make(map[*srcFile]simFile, len(sm.files))
	for f, sf := range sm.files {
		out[f] = *sf
	}
	return out
}

func (sm *sim) restore(snap map[*srcFile]simFile) {
	for f, sf := range snap {
		*sm.files[f] = sf
	}
}

// applyComponent runs the parts of a component step and fills in its
// edits, composed per file from every batch the parts logged, and the
// final text of its sites.
func (sm *sim) applyComponent(st *stepPlan) error {
	p := sm.p
	start := sm.state()
	type window struct {
		f    *srcFile
		a, b int
	}
	windows := make(map[*classified]window)
	for _, c := range st.sites {
		n := p.span(c)
		a, err := sm.cur(c.s.file, p.prog.offset(n.Pos()), false)
		if err != nil {
			continue
		}
		b, err := sm.cur(c.s.file, p.prog.offset(n.End()), true)
		if err != nil {
			continue
		}
		windows[c] = window{c.s.file, a, b}
	}
	for _, part := range st.parts {
		part.out = &Step{}
		if err := sm.apply(part); err != nil {
			return err
		}
	}
	composed := make(map[*srcFile][]intent)
	for _, f := range sm.order {
		sf, s := sm.files[f], start[f]
		if len(sf.log) == s.n {
			continue
		}
		its, err := composeBatches(s.text, sf.text, sf.log[s.n:])
		if err != nil {
			return fmt.Errorf("%s: %w", f.rel, err)
		}
		if len(its) == 0 {
			continue
		}
		composed[f] = its
		st.out.Expect = append(st.out.Expect, FileHash{File: f.rel, SHA256: hash(s.text)})
		for _, it := range its {
			st.out.Edits = append(st.out.Edits, Edit{File: f.rel, Start: it.start, End: it.end, New: it.text})
		}
	}
	for _, c := range st.sites {
		if w, ok := windows[c]; ok && c.owner == nil {
			c.after = finalText(start[w.f].text, composed[w.f], w.a, w.b, p.prog.indentOf(p.span(c).Pos()))
		}
	}
	return nil
}

// preview fills in the before and after of an add-handle, finish or
// component step: per file, the full lines its edits touch, merged where
// they meet.
func (sm *sim) preview(st *stepPlan, before map[*srcFile]simState) {
	if st.kind != stepAdd && st.kind != stepFinish && st.kind != stepComponent {
		return
	}
	var b, a strings.Builder
	for _, fh := range st.out.Expect {
		var f *srcFile
		for _, sf := range sm.order {
			if sf.rel == fh.File {
				f = sf
			}
		}
		var edits []Edit
		for _, e := range st.out.Edits {
			if e.File == fh.File {
				edits = append(edits, e)
			}
		}
		pb, pa := previewLines(before[f].text, edits)
		b.WriteString("// " + fh.File + "\n" + pb)
		a.WriteString("// " + fh.File + "\n" + pa)
	}
	st.out.Before = strings.TrimRight(b.String(), "\n")
	st.out.After = strings.TrimRight(a.String(), "\n")
}

// previewLines returns the full lines of text that sorted edits touch, and
// the same lines once the edits apply; separate groups of lines are joined
// by a "..." line.
func previewLines(text []byte, edits []Edit) (string, string) {
	type rng struct{ s, e int }
	var rs []rng
	for _, e := range edits {
		s, en := e.Start, e.End
		for s > 0 && text[s-1] != '\n' {
			s--
		}
		if en == e.Start || text[en-1] != '\n' {
			for en < len(text) && text[en] != '\n' {
				en++
			}
			if en < len(text) {
				en++
			}
		}
		if n := len(rs); n > 0 && s <= rs[n-1].e {
			rs[n-1].e = max(rs[n-1].e, en)
			continue
		}
		rs = append(rs, rng{s, en})
	}
	var b, a strings.Builder
	for i, r := range rs {
		if i > 0 {
			b.WriteString("...\n")
			a.WriteString("...\n")
		}
		b.Write(text[r.s:r.e])
		last := r.s
		for _, e := range edits {
			if e.Start >= r.s && e.End <= r.e {
				a.Write(text[last:e.Start])
				a.WriteString(e.New)
				last = e.End
			}
		}
		a.Write(text[last:r.e])
	}
	return b.String(), a.String()
}

// finalText returns the text of [a, b) of before once edits apply, widened
// to every edit it reaches, without the indentation of its first line.
func finalText(before []byte, edits []intent, a, b int, indent string) string {
	for changed := true; changed; {
		changed = false
		for _, it := range edits {
			if it.start <= b && it.end >= a && (it.start < a || it.end > b) {
				a, b, changed = min(a, it.start), max(b, it.end), true
			}
		}
	}
	var sb strings.Builder
	last := a
	for _, it := range edits {
		if it.start >= a && it.end <= b {
			sb.Write(before[last:it.start])
			sb.WriteString(it.text)
			last = it.end
		}
	}
	sb.Write(before[last:b])
	lines := strings.Split(strings.TrimRight(sb.String(), "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimPrefix(l, indent)
	}
	lines[0] = strings.TrimLeft(lines[0], " \t")
	return strings.Join(lines, "\n")
}

// resolve turns a step's edits into current coordinates.
func (sm *sim) resolve(st *stepPlan) ([]intent, error) {
	var out []intent
	for _, it := range st.intents {
		s, err := sm.cur(it.file, it.start, it.start != it.end)
		if err != nil {
			return nil, err
		}
		e := s
		if it.end != it.start {
			if e, err = sm.cur(it.file, it.end, false); err != nil {
				return nil, err
			}
		}
		it.start, it.end = s, e
		out = append(out, it)
	}
	if st.current != nil {
		more, err := st.current(sm)
		if err != nil {
			return nil, err
		}
		out = append(out, more...)
	}
	return out, nil
}

// apply runs one step on the simulated files and fills in its edits and
// the hashes the files must have before it.
func (sm *sim) apply(st *stepPlan) error {
	intents, err := sm.resolve(st)
	if err != nil {
		return err
	}
	byFile := make(map[*srcFile][]intent)
	for _, it := range intents {
		byFile[it.file] = append(byFile[it.file], it)
	}
	for _, f := range sm.order {
		its := byFile[f]
		if len(its) == 0 {
			continue
		}
		sf := sm.files[f]
		its, overlap := sortIntents(its)
		if overlap {
			return fmt.Errorf("%s: overlapping edits", f.rel)
		}
		imports, err := importEdits(sf.text, its)
		if err != nil {
			return fmt.Errorf("%s: %w", f.rel, err)
		}
		for i := range imports {
			imports[i].file = f
		}
		its, overlap = sortIntents(append(its, imports...))
		if overlap {
			return fmt.Errorf("%s: import edits overlap code edits", f.rel)
		}
		st.out.Expect = append(st.out.Expect, FileHash{File: f.rel, SHA256: hash(sf.text)})
		pre, clean := sf.text, gofmtClean(sf.text)
		sf.applyBatch(its)
		if clean {
			if fe := formatEdits(sf.text); len(fe) > 0 {
				sf.applyBatch(fe)
				its, err = composeBatches(pre, sf.text, sf.log[len(sf.log)-2:])
				if err != nil {
					return fmt.Errorf("%s: %w", f.rel, err)
				}
			}
		}
		for _, it := range its {
			st.out.Edits = append(st.out.Edits, Edit{File: f.rel, Start: it.start, End: it.end, New: it.text})
		}
	}
	return nil
}

// gofmtClean reports whether src is formatted as gofmt formats it.
func gofmtClean(src []byte) bool {
	f, err := format.Source(src)
	return err == nil && bytes.Equal(f, src)
}

// applyBatch applies non-overlapping edits sorted by start, and moves the
// marks.
func (sf *simFile) applyBatch(its []intent) {
	batch := make([]logEdit, 0, len(its))
	for _, it := range its {
		batch = append(batch, logEdit{start: it.start, end: it.end, n: len(it.text)})
	}
	// Move existing marks. A span survives edits strictly inside it; any
	// other mark an edit reaches into is gone.
	var marks []mark
	for _, m := range sf.marks {
		delta, gone := 0, false
		n := m.n
		for _, e := range batch {
			switch {
			case e.start == e.end:
				if e.start <= m.off {
					delta += e.n
				} else if e.start < m.off+m.n {
					n += e.n
				}
			case e.end <= m.off:
				delta += e.n - (e.end - e.start)
			case e.start >= m.off+m.n:
			case m.kind == markSpan && m.off <= e.start && e.end <= m.off+m.n && e.end-e.start < m.n:
				n += e.n - (e.end - e.start)
			default:
				gone = true
			}
		}
		if !gone {
			m.off += delta
			m.n = n
			marks = append(marks, m)
		}
	}
	// New text, and the marks of inserted text.
	var b strings.Builder
	last, shift := 0, 0
	for _, it := range its {
		b.Write(sf.text[last:it.start])
		newStart := it.start + shift
		for _, m := range it.marks {
			m.off += newStart
			marks = append(marks, m)
		}
		b.WriteString(it.text)
		shift += len(it.text) - (it.end - it.start)
		last = it.end
	}
	b.Write(sf.text[last:])
	sf.text = []byte(b.String())
	sf.log = append(sf.log, batch)
	slices.SortFunc(marks, func(a, b mark) int { return a.off - b.off })
	sf.marks = marks
}

func hash(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// older reports whether version a is older than b (vMAJOR.MINOR.PATCH).
func older(a, b string) bool {
	pa, pb := semverParts(a), semverParts(b)
	if pa == nil || pb == nil {
		return false
	}
	for i := range 3 {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

func semverParts(v string) []int {
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		v = v[:i]
	}
	fs := strings.Split(v, ".")
	if len(fs) != 3 {
		return nil
	}
	out := make([]int, 3)
	for i, s := range fs {
		n := 0
		for _, r := range s {
			if r < '0' || r > '9' {
				return nil
			}
			n = n*10 + int(r-'0')
		}
		out[i] = n
	}
	return out
}
