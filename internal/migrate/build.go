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
	"cmp"
	"fmt"
	"slices"
	"strings"
)

const migrationGuide = "jetstream/MIGRATION.md"

// options configure a plan.
type options struct {
	dir       string
	patterns  []string
	tests     bool
	decisions string
}

// buildPlan loads the packages and plans their migration.
func buildPlan(o options) (*Plan, error) {
	prog, err := load(o.dir, o.patterns, true)
	if err != nil {
		return nil, err
	}
	answers, err := readDecisions(prog.moduleDir, o.decisions)
	if err != nil {
		return nil, err
	}
	sites := inventory(prog)
	g := buildGraph(prog, sites)
	comps := g.components(sites)
	if !o.tests {
		sites, comps = excludeTests(sites, comps)
	}
	p := &planner{prog: prog, g: g, sites: sites, answers: answers}
	compOf := make(map[*site]*component)
	for _, c := range comps {
		for _, s := range c.sites {
			compOf[s] = c
		}
	}
	p.classifyAll(compOf)
	p.cls = slices.DeleteFunc(p.cls, func(c *classified) bool { return c.drop })
	units := p.buildUnits()
	comps = p.mergeComponents(comps, units)
	var pendingComps int
	for _, comp := range comps {
		choice, _ := answers.lookup(patComponent, "", comp.id)
		switch choice {
		case "skip":
			comp.skipped = true
		case "":
			pendingComps++
		}
	}
	for _, c := range p.cls {
		if c.comp != nil && c.comp.skipped {
			c.skipped = true
		}
		if c.owner != nil {
			c.class, c.summary = c.owner.class, "rewritten with the handler of "+c.owner.id
		}
	}
	steps := p.assemble(comps, units)
	plan := &Plan{
		SchemaVersion:     schemaVersion,
		Skill:             "Read `natsvet migrate skill` before acting on this plan",
		Module:            prog.modulePath,
		TableNatsVersion:  tableVersion,
		ModuleNatsVersion: prog.natsVersion,
	}
	p.simulate(plan, steps)
	p.fill(plan, comps, units, steps, pendingComps)
	return plan, nil
}

// excludeTests drops the sites in test files, and marks a component that
// reaches test code as blocked there.
func excludeTests(sites []*site, comps []*component) ([]*site, []*component) {
	var keptSites []*site
	for _, s := range sites {
		if !s.file.test {
			keptSites = append(keptSites, s)
		}
	}
	var keptComps []*component
	for _, c := range comps {
		var inTest, outside bool
		var testSites, other []*site
		for _, s := range c.sites {
			if s.file.test {
				inTest = true
				testSites = append(testSites, s)
			} else {
				outside = true
				other = append(other, s)
			}
		}
		for _, h := range c.handles {
			if h.file.test {
				inTest = true
			} else {
				outside = true
			}
		}
		if !outside {
			continue
		}
		if inTest {
			for _, s := range testSites {
				c.boundaries = append(c.boundaries, boundary{file: s.file, pos: s.anchor.Pos(), reason: "reached from test code, which -tests=false leaves out"})
			}
		}
		c.sites = other
		keptComps = append(keptComps, c)
	}
	return keptSites, keptComps
}

// mergeComponents joins components that one unit spans, and assigns each
// unit its component.
func (p *planner) mergeComponents(comps []*component, units []*unit) []*component {
	into := make(map[*component]*component)
	find := func(c *component) *component {
		for into[c] != nil {
			c = into[c]
		}
		return c
	}
	index := make(map[*component]int)
	for i, c := range comps {
		index[c] = i
	}
	for _, u := range units {
		var first *component
		for _, c := range u.sites {
			if c.comp == nil {
				continue
			}
			cc := find(c.comp)
			switch {
			case first == nil:
				first = cc
			case cc != first:
				a, b := first, cc
				if index[b] < index[a] {
					a, b = b, a
				}
				a.handles = append(a.handles, b.handles...)
				a.roots = append(a.roots, b.roots...)
				a.flows = append(a.flows, b.flows...)
				a.boundaries = append(a.boundaries, b.boundaries...)
				a.sites = append(a.sites, b.sites...)
				into[b] = a
				first = a
			}
		}
	}
	var out []*component
	for _, c := range comps {
		if into[c] == nil {
			out = append(out, c)
		}
	}
	for _, c := range p.cls {
		if c.comp != nil {
			c.comp = find(c.comp)
		}
	}
	for _, u := range units {
		u.comp = nil
		for _, c := range u.sites {
			if c.comp != nil {
				u.comp = c.comp
				break
			}
		}
		for _, c := range u.sites {
			if c.comp == nil {
				c.comp = u.comp
			}
		}
	}
	return out
}

// simulate applies the machine steps in order and numbers every step. A
// step whose edits cannot be applied loses them, and so do the later
// steps of its component.
func (p *planner) simulate(plan *Plan, steps []*stepPlan) {
	sm := newSim(p)
	broken := make(map[*component][]string)
	for i, st := range steps {
		st.out = &Step{ID: fmt.Sprintf("S%d", i), Kind: st.kind, Summary: st.summary, Command: st.command}
		if st.comp != nil {
			st.out.Component = st.comp.id
			if b := broken[st.comp]; len(b) > 0 && st.machine && st.kind != stepSite {
				st.machine = false
				st.waitsOn = append(st.waitsOn, b...)
			}
		}
		if !st.machine {
			continue
		}
		if err := sm.apply(st); err != nil {
			st.machine = false
			st.out.Expect, st.out.Edits = nil, nil
			st.facts = append(st.facts, "the planner could not apply its edits ("+err.Error()+"); make them by hand")
			if st.comp != nil {
				for _, c := range st.sites {
					broken[st.comp] = append(broken[st.comp], c.id)
				}
				if st.kind == stepAdd {
					broken[st.comp] = append(broken[st.comp], "add-handle")
				}
			}
		}
	}
	addStep := make(map[*component]string)
	for _, st := range steps {
		if st.kind == stepAdd {
			addStep[st.comp] = st.out.ID
		}
	}
	for _, st := range steps {
		for i, w := range st.waitsOn {
			if w == "add-handle" && st.comp != nil {
				st.waitsOn[i] = addStep[st.comp]
			}
		}
		st.out.WaitsOn = uniq(st.waitsOn)
		st.out.Facts = uniq(st.facts)
		st.out.Machine = st.machine
		for _, c := range st.sites {
			st.out.Sites = append(st.out.Sites, c.id)
		}
	}
}

// fill writes the sites, components, pending decisions, follow-ups and
// counts into the plan.
func (p *planner) fill(plan *Plan, comps []*component, units []*unit, steps []*stepPlan, pendingComps int) {
	stepOf := make(map[*classified]string)
	for _, st := range steps {
		plan.Steps = append(plan.Steps, *st.out)
		for _, c := range st.sites {
			if _, ok := stepOf[c]; !ok {
				stepOf[c] = st.out.ID
			}
		}
	}
	files := make(map[*srcFile]bool)
	type pendKey struct{ pattern, def, reason string }
	pend := make(map[pendKey][]string)
	var pendOrder []pendKey
	for _, c := range p.cls {
		files[c.s.file] = true
		s := Site{
			ID:       c.id,
			Position: p.pos(c.s.anchor.Pos()),
			Class:    c.class,
			Summary:  c.summary,
			Before:   p.beforeText(c),
			Facts:    c.facts,
			Notes:    uniq(c.notes),
			Step:     stepOf[c],
			Skipped:  c.skipped,
		}
		if c.owner != nil {
			s.Step = stepOf[c.owner]
		}
		if c.comp != nil {
			s.Component = c.comp.id
		}
		if c.ref != "" {
			s.Ref = migrationGuide + c.ref
		}
		values := 0
		for _, u := range c.s.uses {
			if u.value {
				values++
				continue
			}
			s.Symbols = append(s.Symbols, "nats."+u.sym)
		}
		switch {
		case c.class == classMechanical && c.after != "":
			s.After = c.after
		case c.class == classMechanical:
			s.After = p.afterText(c, c.intents)
		case c.class == classGuided:
			s.Template = c.template
		}
		for _, d := range c.decisions {
			sd := SiteDecision{Pattern: d.pattern, Default: d.def, Reason: d.reason, Choice: d.choice}
			for _, o := range patterns[d.pattern].options {
				sd.Options = append(sd.Options, Alternative{ID: o.ID, Summary: o.Summary, After: d.after[o.ID]})
			}
			s.Decisions = append(s.Decisions, sd)
			if d.choice == "" && !c.skipped {
				k := pendKey{d.pattern, d.def, d.reason}
				if _, ok := pend[k]; !ok {
					pendOrder = append(pendOrder, k)
				}
				pend[k] = append(pend[k], c.id)
			}
		}
		if !c.skipped {
			for _, fu := range c.followUps {
				fu.Site = c.id
				plan.FollowUps = append(plan.FollowUps, fu)
			}
		}
		plan.Sites = append(plan.Sites, s)
		plan.Counts.LegacyUses += len(c.s.uses) - values
		plan.Counts.Sites++
		if c.skipped {
			plan.Counts.Skipped++
			continue
		}
		switch c.class {
		case classMechanical:
			plan.Counts.Mechanical++
		case classGuided:
			plan.Counts.Guided++
		case classDecision:
			plan.Counts.Decision++
		case classUnmapped:
			plan.Counts.Unmapped++
		}
	}
	// Pending decisions: per pattern, the most common default at module
	// scope and the others per site.
	byPattern := make(map[string][]pendKey)
	var patOrder []string
	for _, k := range pendOrder {
		if _, ok := byPattern[k.pattern]; !ok {
			patOrder = append(patOrder, k.pattern)
		}
		byPattern[k.pattern] = append(byPattern[k.pattern], k)
	}
	slices.Sort(patOrder)
	for _, pat := range patOrder {
		ks := byPattern[pat]
		slices.SortStableFunc(ks, func(a, b pendKey) int { return cmp.Compare(len(pend[b]), len(pend[a])) })
		var ids []string
		for _, o := range patterns[pat].options {
			ids = append(ids, o.ID)
		}
		for i, k := range ks {
			if i == 0 {
				plan.Pending = append(plan.Pending, Pending{Pattern: pat, Scope: "module", Sites: len(pend[k]), Options: ids, Default: k.def, Reason: k.reason})
				continue
			}
			for _, id := range pend[k] {
				plan.Pending = append(plan.Pending, Pending{Pattern: pat, Scope: "site:" + id, Sites: 1, Options: ids, Default: k.def, Reason: k.reason})
			}
		}
	}
	if pendingComps > 0 {
		var ids []string
		for _, o := range patterns[patComponent].options {
			ids = append(ids, o.ID)
		}
		plan.Pending = append(plan.Pending, Pending{Pattern: patComponent, Scope: "module", Sites: pendingComps, Options: ids, Default: "migrate",
			Reason: "skip keeps code that must stay on the legacy API (tests of the legacy API, compatibility shims) out of the steps; the steps assume migrate until answered"})
	}
	// Components.
	for _, comp := range comps {
		pc := Component{ID: comp.id, Skipped: comp.skipped, OneCommit: comp.oneCommit}
		for _, h := range comp.handles {
			pc.Handles = append(pc.Handles, fmt.Sprintf("%s (%s)", h.ident.Name, p.posID(h.ident.Pos())))
		}
		for _, b := range comp.boundaries {
			pc.Blocked = append(pc.Blocked, Block{Position: p.pos(b.pos), Reason: b.reason})
		}
		for _, st := range steps {
			if st.comp == comp {
				pc.Steps = append(pc.Steps, st.out.ID)
			}
		}
		for _, c := range p.cls {
			if c.comp == comp {
				pc.Sites = append(pc.Sites, c.id)
			}
		}
		plan.Components = append(plan.Components, pc)
	}
	// Files the plan covers.
	for _, st := range plan.Steps {
		for _, e := range st.Edits {
			files[p.prog.fileByRel(e.File)] = true
		}
	}
	for _, f := range p.prog.files {
		if files[f] {
			plan.Files = append(plan.Files, FileHash{File: f.rel, SHA256: hash(f.src)})
		}
	}
	siteIDs, compIDs := make(map[string]bool), make(map[string]bool)
	for _, s := range plan.Sites {
		siteIDs[s.ID] = true
	}
	for _, c := range plan.Components {
		compIDs[c.ID] = true
	}
	plan.StaleAnswers = p.answers.stale(siteIDs, compIDs)
	switch {
	case older(p.prog.natsVersion, tableVersion):
		plan.Notes = append(plan.Notes, fmt.Sprintf("the module requires nats.go %s: apply step S0 and plan again; this plan was classified against the jetstream package of %s", p.prog.natsVersion, p.prog.natsVersion))
	case older(tableVersion, p.prog.natsVersion):
		plan.Notes = append(plan.Notes, fmt.Sprintf("the module requires nats.go %s; the behavior facts in this plan were verified against %s", p.prog.natsVersion, tableVersion))
	}
	if p.prog.js == nil {
		plan.Notes = append(plan.Notes, "the module's nats.go has no jetstream package; apply step S0 and plan again")
	}
	sortPlan(plan)
}

func (prog *program) fileByRel(rel string) *srcFile {
	for _, f := range prog.files {
		if f.rel == rel {
			return f
		}
	}
	return nil
}

// sortPlan orders sites and follow-ups by position.
func sortPlan(plan *Plan) {
	cmpPos := func(a, b Position) int {
		return cmp.Or(strings.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Column, b.Column))
	}
	slices.SortStableFunc(plan.Sites, func(a, b Site) int { return cmpPos(a.Position, b.Position) })
	pos := make(map[string]Position)
	for _, s := range plan.Sites {
		pos[s.ID] = s.Position
	}
	slices.SortStableFunc(plan.FollowUps, func(a, b FollowUp) int { return cmpPos(pos[a.Site], pos[b.Site]) })
}
