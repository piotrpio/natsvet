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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"slices"
	"strings"
)

// Step kinds.
const (
	stepGoGet  = "go-get"
	stepAdd    = "add-handle"
	stepSite   = "site"
	stepRemove = "remove-legacy"
	stepRename = "rename"
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
	out     *Step
}

// compPlan is the steps of one component.
type compPlan struct {
	comp      *component
	add       *stepPlan
	units     []*stepPlan
	remove    *stepPlan
	rename    *stepPlan
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
	if older(p.prog.natsVersion, tableVersion) {
		steps = append(steps, &stepPlan{kind: stepGoGet, machine: false, command: "go get " + natsModule + "@latest",
			summary: fmt.Sprintf("raise nats.go from %s: the mapping was verified against %s, and later v1 releases only add API", p.prog.natsVersion, tableVersion)})
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
		steps = append(steps, cp.add)
		steps = append(steps, cp.units...)
		if cp.remove != nil {
			steps = append(steps, cp.remove, cp.rename)
		}
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
	st := &stepPlan{kind: stepSite, comp: comp, sites: u.sites, machine: true}
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
	if add.machine {
		intents, facts := p.addIntents(comp, cp)
		if len(facts) > 0 {
			add.machine = false
			add.facts = facts
		} else {
			add.intents = intents
		}
	}
	if !add.machine && len(addBlockers) == 0 {
		addBlockers = append(addBlockers, "add-handle")
	}
	var names []string
	for _, h := range comp.handles {
		if h.threadable {
			names = append(names, h.sibling)
		}
	}
	add.summary = "create the jetstream siblings " + strings.Join(names, ", ") + " next to the legacy handles"
	if len(comp.handles) == 0 {
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
		cp.units = append(cp.units, st)
		if !st.machine {
			for _, c := range u.sites {
				if c.role == roleSite && c.class != classMechanical {
					cp.blockers = append(cp.blockers, c.id)
				}
			}
			if len(u.facts) > 0 || len(st.facts) > 0 {
				cp.blockers = append(cp.blockers, u.sites[0].id)
			}
		}
	}
	cp.blockers = uniq(append(append(slices.Clone(blocking), holdBlockers...), cp.blockers...))
	if len(cp.blocked) > 0 {
		return cp
	}
	// Removal and rename.
	rm := &stepPlan{kind: stepRemove, comp: comp, summary: "remove the legacy handles, their roots and the values threaded into them", sites: append(slices.Clone(cp.roots), cp.decls...)}
	rn := &stepPlan{kind: stepRename, comp: comp, summary: "rename each sibling to its legacy name: " + renames(comp)}
	if len(cp.blockers) > 0 {
		rm.waitsOn, rn.waitsOn = cp.blockers, cp.blockers
	} else {
		rm.machine, rn.machine = true, true
		rm.current = func(sm *sim) ([]intent, error) { return p.removeIntents(sm, comp, cp) }
		rn.current = func(sm *sim) ([]intent, error) { return p.renameIntents(sm, comp) }
	}
	cp.remove, cp.rename = rm, rn
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
		if sp, ok := sm.span(h.file, "placeholder:"+string(h.key)); ok {
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

// span returns the current range of a span mark.
func (sm *sim) span(f *srcFile, key string) ([2]int, bool) {
	for _, m := range sm.files[f].marks {
		if m.kind == markSpan && m.key == key {
			return [2]int{m.off, m.off + m.n}, true
		}
	}
	return [2]int{}, false
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
		for _, it := range its {
			st.out.Edits = append(st.out.Edits, Edit{File: f.rel, Start: it.start, End: it.end, New: it.text})
		}
		sf.applyBatch(its)
	}
	return nil
}

// applyBatch applies non-overlapping edits sorted by start, and moves the
// marks.
func (sf *simFile) applyBatch(its []intent) {
	batch := make([]logEdit, 0, len(its))
	for _, it := range its {
		batch = append(batch, logEdit{start: it.start, end: it.end, n: len(it.text)})
	}
	// Move existing marks; drop those inside replaced text.
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
			case e.start < m.off+m.n:
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
