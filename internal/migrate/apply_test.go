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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"

	"github.com/piotrpio/natsvet"
	"github.com/piotrpio/natsvet/analyzers/legacyjs"
)

// copyModule copies the go.mod and go.sum of src and the given
// subdirectories to a new temporary module directory.
func copyModule(t *testing.T, src string, dirs ...string) string {
	t.Helper()
	dst := t.TempDir()
	for _, name := range []string{"go.mod", "go.sum"} {
		b, err := os.ReadFile(filepath.Join(src, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dst, name), b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, d := range dirs {
		root := filepath.Join(src, d)
		err := filepath.WalkDir(root, func(path string, e fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(src, path)
			if e.IsDir() {
				if e.Name() == "testdata" || (path != root && fileExists(filepath.Join(path, "go.mod"))) {
					return filepath.SkipDir
				}
				return os.MkdirAll(filepath.Join(dst, rel), 0o755)
			}
			b, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	return dst
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// planWithDefaults plans dir, answering every pending decision with its
// default in the module's natsvet-migrate.json, until none is pending.
func planWithDefaults(t *testing.T, dir string, patterns []string) *Plan {
	t.Helper()
	var answers []Answer
	for range 4 {
		d := Decisions{Version: 1, Answers: answers}
		b, _ := json.MarshalIndent(d, "", "  ")
		if err := os.WriteFile(filepath.Join(dir, decisionsFile), b, 0o644); err != nil {
			t.Fatal(err)
		}
		plan, err := buildPlan(options{dir: dir, patterns: patterns, tests: true})
		if err != nil {
			t.Fatal(err)
		}
		if len(plan.Pending) == 0 {
			return plan
		}
		for _, pd := range plan.Pending {
			answers = append(answers, Answer{Pattern: pd.Pattern, Scope: pd.Scope, Choice: pd.Default})
		}
	}
	t.Fatal("decisions still pending after answering every default")
	return nil
}

// applyStep applies one machine step to the module in dir, refusing a file
// whose hash differs from the step's.
func applyStep(dir string, st Step) error {
	for _, e := range st.Expect {
		b, err := os.ReadFile(filepath.Join(dir, e.File))
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
		path := filepath.Join(dir, file)
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sort.SliceStable(edits, func(i, j int) bool { return edits[i].Start > edits[j].Start })
		for _, e := range edits {
			b = append(b[:e.Start:e.Start], append([]byte(e.New), b[e.End:]...)...)
		}
		if err := os.WriteFile(path, b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// typeErrors type-checks the packages of dir and returns their errors.
func typeErrors(dir string, patterns ...string) []string {
	cfg := &packages.Config{Mode: packages.LoadAllSyntax, Dir: dir, Tests: true, Fset: token.NewFileSet(), Env: append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off")}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return []string{err.Error()}
	}
	var out []string
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			out = append(out, e.Error())
		}
	})
	slices.Sort(out)
	return slices.Compact(out)
}

var legacyMsg = regexp.MustCompile(`legacy JetStream API: (\S+);`)

// legacyResidue returns the legacy symbols legacyjs reports in dir, as
// "file: symbol" lines.
func legacyResidue(t *testing.T, dir string, patterns ...string) []string {
	t.Helper()
	cfg := &packages.Config{Mode: packages.LoadAllSyntax, Dir: dir, Tests: true, Fset: token.NewFileSet(), Env: append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off")}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		t.Fatal(err)
	}
	if err := legacyjs.Analyzer.Flags.Set("enable", "true"); err != nil {
		t.Fatal(err)
	}
	defer legacyjs.Analyzer.Flags.Set("enable", "false")
	graph, err := checker.Analyze([]*analysis.Analyzer{legacyjs.Analyzer}, pkgs, nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	var out []string
	for act := range graph.All() {
		if act.Analyzer != legacyjs.Analyzer {
			continue
		}
		for _, d := range act.Diagnostics {
			pos := cfg.Fset.Position(d.Pos)
			key := pos.String()
			if seen[key] {
				continue
			}
			seen[key] = true
			rel, _ := filepath.Rel(dir, pos.Filename)
			m := legacyMsg.FindStringSubmatch(d.Message)
			if m == nil {
				t.Fatalf("unexpected legacyjs message %q", d.Message)
			}
			out = append(out, filepath.ToSlash(rel)+": "+m[1])
		}
	}
	slices.Sort(out)
	return out
}

// expectedResidue returns the legacy uses of the sites the applied steps
// did not migrate, as "file: symbol" lines.
func expectedResidue(plan *Plan, applied map[string]bool) []string {
	steps := make(map[string]Step)
	for _, st := range plan.Steps {
		steps[st.ID] = st
	}
	removed := make(map[string]bool) // components whose removal applied
	for _, st := range plan.Steps {
		if st.Kind == stepRemove && applied[st.ID] {
			removed[st.Component] = true
		}
	}
	var out []string
	for _, s := range plan.Sites {
		migrated := false
		st, ok := steps[s.Step]
		switch {
		case !ok || s.Skipped:
		case st.Kind == stepAdd:
			migrated = removed[s.Component]
		default:
			migrated = applied[st.ID]
		}
		if migrated {
			continue
		}
		for _, sym := range s.Symbols {
			out = append(out, s.Position.File+": "+legacyName(sym))
		}
	}
	slices.Sort(out)
	return out
}

// legacyName renders a site symbol as legacyjs names it: methods are
// qualified by the type that declares them.
func legacyName(sym string) string { return sym }

// applyAll applies every machine step of plan in order, type-checking
// after each, and returns the applied step ids.
func applyAll(t *testing.T, dir string, plan *Plan, patterns ...string) map[string]bool {
	t.Helper()
	applied := make(map[string]bool)
	for _, st := range plan.Steps {
		if !st.Machine {
			continue
		}
		if err := applyStep(dir, st); err != nil {
			t.Fatal(err)
		}
		if errs := typeErrors(dir, patterns...); len(errs) > 0 {
			var files []string
			for _, e := range st.Edits {
				files = append(files, e.File)
			}
			var dump strings.Builder
			for _, f := range uniq(files) {
				b, _ := os.ReadFile(filepath.Join(dir, f))
				fmt.Fprintf(&dump, "--- %s\n%s\n", f, b)
			}
			t.Fatalf("after step %s (%s: %s) the module does not type-check:\n%s\n%s", st.ID, st.Kind, st.Summary, strings.Join(errs, "\n"), dump.String())
		}
		applied[st.ID] = true
	}
	return applied
}

func TestApplyTestdata(t *testing.T) {
	pinnedJetStream(t)
	dir := copyModule(t, testdataDir, "migrate", "migrateext")
	patterns := []string{"./migrate/..."}
	plan := planWithDefaults(t, dir, patterns)
	applied := applyAll(t, dir, plan, patterns...)
	if len(applied) == 0 {
		t.Fatal("no step carried machine edits")
	}
	got := legacyResidue(t, dir, patterns...)
	want := expectedResidue(plan, applied)
	if !slices.Equal(got, want) {
		t.Errorf("legacy uses after applying the plan differ from the sites it did not migrate\ngot:  %v\nwant: %v", got, want)
	}
	before := ruleReports(t, copyModule(t, testdataDir, "migrate", "migrateext"), patterns...)
	if after := ruleReports(t, dir, patterns...); !slices.Equal(after, before) {
		t.Errorf("the other natsvet rules report differently after the migration\nbefore: %v\nafter:  %v", before, after)
	}
}

// ruleReports returns the messages of every default natsvet rule other
// than legacyjs over dir, sorted.
func ruleReports(t *testing.T, dir string, patterns ...string) []string {
	t.Helper()
	cfg := &packages.Config{Mode: packages.LoadAllSyntax, Dir: dir, Tests: true, Fset: token.NewFileSet(), Env: append(os.Environ(), "GOFLAGS=-mod=mod", "GOPROXY=off")}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		t.Fatal(err)
	}
	var analyzers []*analysis.Analyzer
	for _, a := range natsvet.All() {
		if a != legacyjs.Analyzer {
			analyzers = append(analyzers, a)
		}
	}
	graph, err := checker.Analyze(analyzers, pkgs, nil)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for act := range graph.All() {
		if !act.IsRoot {
			continue
		}
		for _, d := range act.Diagnostics {
			rel, _ := filepath.Rel(dir, cfg.Fset.Position(d.Pos).Filename)
			out = append(out, filepath.ToSlash(rel)+": "+act.Analyzer.Name+": "+d.Message)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

func TestApplyRefusesChangedFile(t *testing.T) {
	pinnedJetStream(t)
	dir := copyModule(t, testdataDir, "migrate", "migrateext")
	plan := planWithDefaults(t, dir, []string{"./migrate/..."})
	var first Step
	for _, st := range plan.Steps {
		if st.Machine {
			first = st
			break
		}
	}
	if len(first.Expect) == 0 {
		t.Fatal("no machine step with expected hashes")
	}
	path := filepath.Join(dir, first.Expect[0].File)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(b, "\n// edited by hand\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := applyStep(dir, first); err == nil || !strings.Contains(err.Error(), "changed since the plan") {
		t.Errorf("applying a step over an edited file: err = %v, want a hash refusal", err)
	}
}

// copyTree copies a module directory, without its VCS data, to a new
// temporary directory.
func copyTree(t *testing.T, src string) string {
	t.Helper()
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		if e.IsDir() {
			if e.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		if !e.Type().IsRegular() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

// TestApplyCorpus plans a corpus repository (natscli unless
// NATSVET_MIGRATE_REPO names another) from the corpus cache with default
// answers, applies every machine step with a type-check after each, and
// checks the residue and the other rules, as TestApplyTestdata does.
func TestApplyCorpus(t *testing.T) {
	corpus := os.Getenv("NATSVET_MIGRATE_CORPUS")
	if corpus == "" {
		t.Skip("NATSVET_MIGRATE_CORPUS is not set")
	}
	repo := cmp.Or(os.Getenv("NATSVET_MIGRATE_REPO"), "natscli")
	dir := copyTree(t, filepath.Join(corpus, repo))
	patterns := []string{"./..."}
	plan := planWithDefaults(t, dir, patterns)
	t.Logf("%s: %+v", repo, plan.Counts)
	before := ruleReports(t, copyTree(t, filepath.Join(corpus, repo)), patterns...)
	applied := applyAll(t, dir, plan, patterns...)
	t.Logf("applied %d of %d steps", len(applied), len(plan.Steps))
	got := legacyResidue(t, dir, patterns...)
	want := expectedResidue(plan, applied)
	if !slices.Equal(got, want) {
		t.Errorf("legacy uses after applying the plan differ from the sites it did not migrate\ngot:  %v\nwant: %v", got, want)
	}
	if after := ruleReports(t, dir, patterns...); !slices.Equal(after, before) {
		t.Errorf("the other natsvet rules report differently after the migration\nbefore: %v\nafter:  %v", before, after)
	}
	out := os.Getenv("NATSVET_MIGRATE_PLAN_OUT")
	if out != "" {
		f, err := os.Create(out)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := writeMarkdown(f, plan); err != nil {
			t.Fatal(err)
		}
	}
}
