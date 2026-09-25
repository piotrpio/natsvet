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
	"errors"
	"flag"
	"fmt"
	"go/token"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// applyHook, when set by a test, may change a step before apply writes it.
var applyHook func(*Step)

// Exit codes of apply beyond 0 (applied, or nothing left) and 1 (an error,
// a refusal or a rollback).
const exitStopped = 3 // the next step needs a person; nothing was written

// tree holds files of a module being edited step by step: their current
// content and, for every file read, the content before the first step.
type tree struct {
	dir         string
	files, orig map[string][]byte
}

func newTree(dir string) *tree {
	return &tree{dir: dir, files: make(map[string][]byte), orig: make(map[string][]byte)}
}

func (tr *tree) read(rel string) ([]byte, error) {
	if b, ok := tr.files[rel]; ok {
		return b, nil
	}
	b, err := os.ReadFile(filepath.Join(tr.dir, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	tr.files[rel], tr.orig[rel] = b, b
	return b, nil
}

// splice applies a step's edits, refusing a file whose content does not
// have the hash the step records for it; nothing changes on a refusal.
func (tr *tree) splice(st Step) error {
	for _, e := range st.Expect {
		b, err := tr.read(e.File)
		if err != nil {
			return err
		}
		h := sha256.Sum256(b)
		if got := hex.EncodeToString(h[:]); got != e.SHA256 {
			return fmt.Errorf("step %s: %s changed since the plan (sha256 %s, want %s)", st.ID, e.File, got, e.SHA256)
		}
	}
	byFile := make(map[string][]Edit)
	for _, e := range st.Edits {
		byFile[e.File] = append(byFile[e.File], e)
	}
	for file, edits := range byFile {
		b, err := tr.read(file)
		if err != nil {
			return err
		}
		sort.SliceStable(edits, func(i, j int) bool { return edits[i].Start > edits[j].Start })
		for _, e := range edits {
			b = append(b[:e.Start:e.Start], append([]byte(e.New), b[e.End:]...)...)
		}
		tr.files[file] = b
	}
	return nil
}

// changed returns the files whose content differs from before the first
// step, sorted.
func (tr *tree) changed() []string {
	var out []string
	for rel, b := range tr.files {
		if string(b) != string(tr.orig[rel]) {
			out = append(out, rel)
		}
	}
	slices.Sort(out)
	return out
}

// write saves the changed files.
func (tr *tree) write() error {
	for _, rel := range tr.changed() {
		if err := os.WriteFile(filepath.Join(tr.dir, filepath.FromSlash(rel)), tr.files[rel], 0o644); err != nil {
			return err
		}
	}
	return nil
}

// restore saves every changed file's content from before the first step.
func (tr *tree) restore() error {
	var errs []error
	for _, rel := range tr.changed() {
		errs = append(errs, os.WriteFile(filepath.Join(tr.dir, filepath.FromSlash(rel)), tr.orig[rel], 0o644))
	}
	return errors.Join(errs...)
}

// applyFlags are the flags of migrate apply: those of plan, and the
// component and dry-run selection.
type applyFlags struct {
	*planFlags
	component string
	dryRun    bool
}

func newApplyFlags(stderr io.Writer) (*flag.FlagSet, *applyFlags) {
	fs := flag.NewFlagSet("natsvet migrate apply", flag.ContinueOnError)
	fs.SetOutput(stderr)
	af := &applyFlags{planFlags: &planFlags{}}
	fs.StringVar(&af.decisions, "decisions", "", "answers file (default: "+decisionsFile+" at the module root, when it exists)")
	fs.BoolVar(&af.tests, "tests", true, "include test files")
	fs.StringVar(&af.component, "component", "", "apply this component's machine steps, up to its first step that is not one")
	fs.BoolVar(&af.dryRun, "dry-run", false, "print the edits as a unified diff and write nothing")
	return fs, af
}

// runApply plans the packages in dir and applies the next machine step, or
// a component's machine steps, and returns the exit code.
func runApply(dir string, af *applyFlags, patterns []string, stdout, stderr io.Writer) int {
	plan, err := buildPlan(options{dir: dir, patterns: patterns, tests: af.tests, decisions: af.decisions})
	if err != nil {
		fmt.Fprintf(stderr, "%s migrate apply: %v\n", progname(), err)
		return 1
	}
	if len(plan.Steps) > 0 && plan.Steps[0].Kind == stepGoGet {
		st := plan.Steps[0]
		fmt.Fprintf(stdout, "stopped: step %s comes first: %s. Run\n\n    %s\n\nthen build, and apply again.\n", st.ID, st.Summary, st.Command)
		return exitStopped
	}
	var steps []Step
	var stop *Step
	if af.component != "" {
		if !slices.ContainsFunc(plan.Components, func(c Component) bool { return c.ID == af.component }) {
			var ids []string
			for _, c := range plan.Components {
				ids = append(ids, c.ID)
			}
			fmt.Fprintf(stderr, "%s migrate apply: no component %s; the plan has %s\n", progname(), af.component, strings.Join(ids, ", "))
			return 1
		}
		for i, st := range plan.Steps {
			if st.Component != af.component {
				continue
			}
			if !st.Machine {
				stop = &plan.Steps[i]
				break
			}
			steps = append(steps, st)
		}
	} else {
		for _, st := range plan.Steps {
			if st.Machine {
				steps = append(steps, st)
				break
			}
		}
	}
	if len(steps) == 0 {
		if stop != nil {
			fmt.Fprintf(stdout, "stopped: %s\n", stopReason(plan, *stop))
			return exitStopped
		}
		fmt.Fprintln(stdout, "nothing left to apply.")
		remaining(stdout, plan)
		return 0
	}
	for _, pd := range plan.Pending {
		for _, st := range steps {
			if pd.Pattern == patComponent && slices.Contains(pd.Components, st.Component) {
				fmt.Fprintf(stderr, "%s migrate apply: the user has not decided whether to migrate component %s: ask them to migrate or skip it, and record the answer in %s (pattern %q, scope \"component:%s\" or \"module\")\n", progname(), st.Component, decisionsFile, patComponent, st.Component)
				return 1
			}
		}
	}
	tr := newTree(dir)
	for i := range steps {
		if applyHook != nil {
			applyHook(&steps[i])
		}
		before := make(map[string][]byte)
		for _, e := range steps[i].Expect {
			if b, err := tr.read(e.File); err == nil {
				before[e.File] = b
			}
		}
		if err := tr.splice(steps[i]); err != nil {
			fmt.Fprintf(stderr, "%s migrate apply: %v\n", progname(), err)
			return 1
		}
		if af.dryRun {
			for _, e := range steps[i].Expect {
				var edits []Edit
				for _, ed := range steps[i].Edits {
					if ed.File == e.File {
						edits = append(edits, ed)
					}
				}
				fmt.Fprint(stdout, unifiedDiff(e.File, before[e.File], edits))
			}
		}
	}
	if af.dryRun {
		return 0
	}
	if err := tr.write(); err != nil {
		fmt.Fprintf(stderr, "%s migrate apply: %v\n", progname(), err)
		if rerr := tr.restore(); rerr != nil {
			fmt.Fprintf(stderr, "%s migrate apply: restoring the files: %v\n", progname(), rerr)
		}
		return 1
	}
	if errs := typeCheck(dir, tr.changed(), af.tests); len(errs) > 0 {
		var ids []string
		for _, st := range steps {
			ids = append(ids, st.ID)
		}
		fmt.Fprintf(stderr, "%s migrate apply: after step %s the packages do not type-check, so the files are restored; this is a planner bug:\n\t%s\n", progname(), strings.Join(ids, ", "), strings.Join(errs, "\n\t"))
		if err := tr.restore(); err != nil {
			fmt.Fprintf(stderr, "%s migrate apply: restoring the files: %v\n", progname(), err)
		}
		return 1
	}
	sites := make(map[string]Site)
	for _, s := range plan.Sites {
		sites[s.ID] = s
	}
	for _, st := range steps {
		fmt.Fprintf(stdout, "applied %s (%s%s): %s\n", st.ID, st.Kind, ofComponent(st), st.Summary)
		var files, fns []string
		for _, e := range st.Expect {
			files = append(files, e.File)
		}
		for _, id := range st.Sites {
			if f := sites[id].Function; f != "" && !slices.Contains(fns, f) {
				fns = append(fns, f)
			}
		}
		fmt.Fprintf(stdout, "  files: %s\n", strings.Join(files, ", "))
		if len(fns) > 0 {
			fmt.Fprintf(stdout, "  functions: %s\n", strings.Join(fns, ", "))
		}
	}
	last := slices.IndexFunc(plan.Steps, func(st Step) bool { return st.ID == steps[len(steps)-1].ID })
	switch {
	case stop != nil:
		fmt.Fprintf(stdout, "stopped: %s\n", stopReason(plan, *stop))
	case last+1 < len(plan.Steps):
		next := plan.Steps[last+1]
		fmt.Fprintf(stdout, "next: %s (%s%s%s): %s\n", next.ID, next.Kind, map[bool]string{true: ", machine"}[next.Machine], ofComponent(next), next.Summary)
	default:
		fmt.Fprintln(stdout, "next: none; plan again to see what is left")
	}
	return 0
}

// ofComponent names a step's component for apply's output: step ids number
// the current plan only, component ids stay.
func ofComponent(st Step) string {
	if st.Component == "" {
		return ""
	}
	return " of component " + st.Component
}

// stopReason says why a step needs a person.
func stopReason(plan *Plan, st Step) string {
	why := "it is done by hand"
	switch {
	case len(st.WaitsOn) > 0:
		why = "it waits on " + strings.Join(st.WaitsOn, ", ")
	case len(st.Sites) > 0:
		var parts []string
		for _, s := range plan.Sites {
			if slices.Contains(st.Sites, s.ID) && s.Class != classMechanical {
				parts = append(parts, s.ID+" is "+s.Class)
			}
		}
		if len(parts) > 0 {
			why = strings.Join(parts, "; ")
		}
	}
	if len(st.Facts) > 0 {
		why += " (" + strings.Join(st.Facts, "; ") + ")"
	}
	return fmt.Sprintf("step %s (%s%s) is not a machine step: %s. Migrate it by hand as the plan shows, then apply again.", st.ID, st.Kind, ofComponent(st), why)
}

// remaining lists the sites a plan with no machine step still holds.
func remaining(w io.Writer, plan *Plan) {
	for _, s := range plan.Sites {
		if s.Skipped {
			continue
		}
		fmt.Fprintf(w, "  %s (%s): %s\n", s.ID, s.Class, s.Summary)
	}
}

// typeCheck loads the packages holding files, tests included when tests
// is set, and returns their errors.
func typeCheck(dir string, files []string, tests bool) []string {
	var patterns []string
	for _, f := range files {
		p := "./" + path.Dir(f)
		if !slices.Contains(patterns, p) {
			patterns = append(patterns, p)
		}
	}
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedFiles | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo | packages.NeedImports, Dir: dir, Tests: tests, Fset: token.NewFileSet()}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return []string{err.Error()}
	}
	var out []string
	for _, p := range pkgs {
		for _, e := range p.Errors {
			out = append(out, e.Error())
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// unifiedDiff renders a step's edits of one file, sorted by offset, as a
// unified diff against before.
func unifiedDiff(file string, before []byte, edits []Edit) string {
	if len(edits) == 0 {
		return ""
	}
	starts := []int{0}
	for i, c := range before {
		if c == '\n' && i+1 < len(before) {
			starts = append(starts, i+1)
		}
	}
	lineOf := func(off int) int {
		return sort.Search(len(starts), func(i int) bool { return starts[i] > off }) - 1
	}
	offOf := func(line int) int {
		if line >= len(starts) {
			return len(before)
		}
		return starts[line]
	}
	sorted := slices.Clone(edits)
	slices.SortFunc(sorted, func(a, b Edit) int { return a.Start - b.Start })
	// Regions of whole lines the edits replace.
	type region struct{ from, to int }
	var regions []region
	for _, e := range sorted {
		r := region{lineOf(e.Start), lineOf(e.Start) + 1}
		switch {
		case e.Start == e.End && e.Start == offOf(r.from) && strings.HasSuffix(e.New, "\n"):
			r.to = r.from
		case e.End > e.Start && e.End == offOf(lineOf(e.End)):
			r.to = lineOf(e.End)
		default:
			r.to = lineOf(max(e.End, e.Start)) + 1
		}
		if n := len(regions); n > 0 && r.from <= regions[n-1].to {
			regions[n-1].to = max(regions[n-1].to, r.to)
			continue
		}
		regions = append(regions, r)
	}
	splitLines := func(s string) []string {
		if s == "" {
			return nil
		}
		return strings.SplitAfter(strings.TrimSuffix(s, "\n"), "\n")
	}
	lines := splitLines(string(before))
	const context = 3
	var b strings.Builder
	fmt.Fprintf(&b, "--- a/%s\n+++ b/%s\n", file, file)
	delta := 0
	for i := 0; i < len(regions); {
		// A hunk: regions whose gaps are within twice the context.
		j := i
		for j+1 < len(regions) && regions[j+1].from-regions[j].to <= 2*context {
			j++
		}
		from, to := max(0, regions[i].from-context), min(len(lines), regions[j].to+context)
		var body strings.Builder
		oldN, newN := 0, 0
		cur := from
		for k := i; k <= j; k++ {
			r := regions[k]
			for ; cur < r.from; cur++ {
				body.WriteString(" " + ensureNL(lines[cur]))
				oldN++
				newN++
			}
			seg := string(before[offOf(r.from):offOf(r.to)])
			after := seg
			shift := 0
			for _, e := range sorted {
				if e.Start >= offOf(r.from) && e.End <= offOf(r.to) && !(e.Start == e.End && e.Start == offOf(r.to) && r.to > r.from) {
					s, en := e.Start-offOf(r.from)+shift, e.End-offOf(r.from)+shift
					after = after[:s] + e.New + after[en:]
					shift += len(e.New) - (e.End - e.Start)
				}
			}
			for _, l := range splitLines(seg) {
				body.WriteString("-" + ensureNL(l))
				oldN++
			}
			for _, l := range splitLines(after) {
				body.WriteString("+" + ensureNL(l))
				newN++
			}
			cur = r.to
		}
		for ; cur < to; cur++ {
			body.WriteString(" " + ensureNL(lines[cur]))
			oldN++
			newN++
		}
		fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n%s", from+1, oldN, from+1+delta, newN, body.String())
		delta += newN - oldN
		i = j + 1
	}
	return b.String()
}

func ensureNL(s string) string {
	if strings.HasSuffix(s, "\n") {
		return s
	}
	return s + "\n"
}
