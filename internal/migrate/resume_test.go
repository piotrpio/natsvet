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
	"go/format"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// writeGuided replaces the lines from "// guided: begin" to "// guided: end"
// of the file at path with fixture, as an agent migrating a guided site by
// hand would.
func writeGuided(t *testing.T, path string, fixture []byte) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	begin, end := []byte("\t// guided: begin\n"), []byte("\t// guided: end\n")
	i, j := bytes.Index(b, begin), bytes.Index(b, end)
	if i < 0 || j < i {
		t.Fatalf("%s has no guided region", path)
	}
	if err := os.WriteFile(path, slices.Concat(b[:i], fixture, b[j+len(end):]), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestResumeAfterGuidedEdit applies a component's machine steps, migrates
// its guided site by hand, plans again and finishes the component.
func TestResumeAfterGuidedEdit(t *testing.T) {
	pinnedJetStream(t)
	dir := copyModule(t, testdataDir, "migrate", "migrateext")
	patterns := []string{"./migrate/guided"}
	file := filepath.Join(dir, "migrate/guided/guided.go")
	applyAll(t, dir, planWithDefaults(t, dir, patterns), patterns...)
	fixture, err := os.ReadFile(filepath.Join("testdata", "guided.fixture"))
	if err != nil {
		t.Fatal(err)
	}
	writeGuided(t, file, fixture)
	if errs := typeErrors(dir, patterns...); len(errs) > 0 {
		t.Fatalf("the hand-migrated site does not type-check: %v", errs)
	}
	again := planWithDefaults(t, dir, patterns)
	for _, st := range again.Steps {
		if st.Kind == stepAdd {
			t.Errorf("after the hand edit the plan adds the siblings again in %s", st.ID)
		}
		if !st.Machine {
			t.Errorf("after the hand edit step %s (%s) is not a machine step (waits on %v, facts %v)", st.ID, st.Kind, st.WaitsOn, st.Facts)
		}
	}
	for _, c := range again.Components {
		if len(c.Blocked) > 0 {
			t.Errorf("after the hand edit component %s is blocked: %+v", c.ID, c.Blocked)
		}
	}
	if t.Failed() {
		return
	}
	applyAll(t, dir, again, patterns...)
	if got := legacyResidue(t, dir, patterns...); len(got) > 0 {
		t.Errorf("legacy uses left after finishing: %v", got)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	golden(t, "guided.final.go.txt", b)
}

// applyMachine applies every machine step of plan in order, from the step
// after skip on, without type-checking in between.
func applyMachine(t *testing.T, dir string, plan *Plan, skip int) {
	t.Helper()
	for _, st := range plan.Steps[skip:] {
		if !st.Machine {
			continue
		}
		if err := applyStep(dir, st); err != nil {
			t.Fatal(err)
		}
	}
}

// goFiles returns the content of every .go file under dir/sub, by path
// relative to dir.
func goFiles(t *testing.T, dir, sub string) map[string]string {
	t.Helper()
	out := make(map[string]string)
	err := filepath.WalkDir(filepath.Join(dir, sub), func(path string, e os.DirEntry, err error) error {
		if err != nil || e.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		out[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestResumeAfterAddHandle checks, after every add-handle step of the
// testdata plan, that planning again and applying the new plan gives the
// files the uninterrupted plan gives.
func TestResumeAfterAddHandle(t *testing.T) {
	pinnedJetStream(t)
	patterns := []string{"./migrate/..."}
	base := copyModule(t, testdataDir, "migrate", "migrateext")
	plan := planWithDefaults(t, base, patterns)
	applyMachine(t, base, plan, 0)
	want := goFiles(t, base, "migrate")
	for i, st := range plan.Steps {
		if st.Kind != stepAdd || !st.Machine {
			continue
		}
		t.Run(st.ID, func(t *testing.T) {
			dir := copyModule(t, testdataDir, "migrate", "migrateext")
			for _, prev := range plan.Steps[:i+1] {
				if prev.Machine {
					if err := applyStep(dir, prev); err != nil {
						t.Fatal(err)
					}
				}
			}
			again := planWithDefaults(t, dir, patterns)
			applyMachine(t, dir, again, 0)
			if errs := typeErrors(dir, patterns...); len(errs) > 0 {
				t.Fatalf("resumed after %s, the module does not type-check: %v", st.ID, errs)
			}
			got := goFiles(t, dir, "migrate")
			for path, w := range want {
				if got[path] != w {
					t.Errorf("resumed after %s (component %s), %s differs from the uninterrupted plan:\n%s", st.ID, st.Component, path, got[path])
				}
			}
		})
	}
}

// TestNoFormattingOfUnformattedFile checks that a file that is not
// gofmt-clean gets no formatting edits: no step replaces whitespace with
// whitespace in it.
func TestNoFormattingOfUnformattedFile(t *testing.T) {
	pinnedJetStream(t)
	dir := copyModule(t, testdataDir, "migrate", "migrateext")
	const rel = "migrate/fieldfmt/fieldfmt.go"
	path := filepath.Join(dir, rel)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	b = bytes.Replace(b, []byte("\tNC   *nats.Conn"), []byte("\tNC *nats.Conn"), 1)
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	patterns := []string{"./migrate/fieldfmt/..."}
	for _, st := range planWithDefaults(t, dir, patterns).Steps {
		if !st.Machine {
			continue
		}
		cur, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range st.Edits {
			if e.File == rel && len(bytes.TrimSpace(cur[e.Start:e.End])) == 0 && len(bytes.TrimSpace([]byte(e.New))) == 0 && (e.End > e.Start || e.New != "") {
				t.Errorf("step %s edits whitespace only in the unformatted file: %q -> %q", st.ID, cur[e.Start:e.End], e.New)
			}
		}
		if err := applyStep(dir, st); err != nil {
			t.Fatal(err)
		}
	}
}

// TestResumeAfterHandEdits applies add-handle, edits around the siblings by
// hand (a line inserted before the local sibling, the sibling field moved
// to the end of its struct, gofmt), plans again and finishes.
func TestResumeAfterHandEdits(t *testing.T) {
	pinnedJetStream(t)
	dir := copyModule(t, testdataDir, "migrate", "migrateext")
	patterns := []string{"./migrate/fieldfmt/..."}
	plan := planWithDefaults(t, dir, patterns)
	if st := plan.Steps[0]; st.Kind != stepAdd || !st.Machine {
		t.Fatalf("first step %s is %s (machine %v), want a machine add-handle", st.ID, st.Kind, st.Machine)
	}
	if err := applyStep(dir, plan.Steps[0]); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "migrate/fieldfmt/fieldfmt.go")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, r := range []struct{ old, new string }{
		{"\tjsNew, err := jetstream.New(nc)\n", "\tprintln(\"connected\")\n\tjsNew, err := jetstream.New(nc)\n"},
		{"\tJSNew jetstream.JetStream\n", ""},
		{"\tName  string\n}", "\tName  string\n\tJSNew jetstream.JetStream\n}"},
	} {
		if !strings.Contains(s, r.old) {
			t.Fatalf("fieldfmt.go after add-handle has no %q:\n%s", r.old, s)
		}
		s = strings.Replace(s, r.old, r.new, 1)
	}
	formatted, err := format.Source([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		t.Fatal(err)
	}
	again := planWithDefaults(t, dir, patterns)
	for _, st := range again.Steps {
		if st.Kind == stepAdd && strings.HasPrefix(st.Component, "migrate/fieldfmt/fieldfmt.go") {
			t.Errorf("after the hand edits the plan adds the siblings again in %s (%v)", st.ID, st.Facts)
		}
	}
	applyAll(t, dir, again, patterns...)
	if got := legacyResidue(t, dir, patterns...); len(got) > 0 {
		t.Errorf("legacy uses left after finishing: %v", got)
	}
	final, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(final), "JSNew") || strings.Contains(string(final), "jsNew") {
		t.Errorf("a sibling name is left:\n%s", final)
	}
}

// TestUnrelatedJetStreamVariable checks that a jetstream handle named like
// a sibling, with no legacy handle in its scope, is left alone.
func TestUnrelatedJetStreamVariable(t *testing.T) {
	pinnedJetStream(t)
	dir := copyModule(t, testdataDir, "migrate", "migrateext")
	patterns := []string{"./migrate/unrelated"}
	path := filepath.Join(dir, "migrate/unrelated/unrelated.go")
	native := func() string {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		s := string(b)
		return s[strings.Index(s, "func Native"):strings.Index(s, "func Legacy")]
	}
	before := native()
	applyAll(t, dir, planWithDefaults(t, dir, patterns), patterns...)
	if got := legacyResidue(t, dir, patterns...); len(got) > 0 {
		t.Errorf("legacy uses left: %v", got)
	}
	if after := native(); after != before {
		t.Errorf("the jetstream function changed:\n%s\nwant:\n%s", after, before)
	}
}

// TestOneCommitComponentStep checks that a component safe in one commit
// gets one step, whose sites show their final text and whose edits leave
// no sibling or placeholder behind.
func TestOneCommitComponentStep(t *testing.T) {
	pinnedJetStream(t)
	dir := copyModule(t, testdataDir, "migrate", "migrateext")
	patterns := []string{"./migrate/independent", "./migrate/multifile"}
	plan := planWithDefaults(t, dir, patterns)
	steps := make(map[string]Step)
	for _, st := range plan.Steps {
		steps[st.ID] = st
	}
	if len(plan.Components) != 3 {
		t.Fatalf("got %d components, want 3", len(plan.Components))
	}
	for _, c := range plan.Components {
		if !c.OneCommit {
			t.Errorf("component %s is not one-commit", c.ID)
			continue
		}
		if len(c.Steps) != 1 || steps[c.Steps[0]].Kind != stepComponent || !steps[c.Steps[0]].Machine {
			t.Errorf("component %s has steps %v, want one machine component step", c.ID, c.Steps)
		}
	}
	for _, s := range plan.Sites {
		if strings.Contains(s.After, "New") && !strings.Contains(s.After, "jetstream.New") {
			t.Errorf("site %s shows a sibling in its after text:\n%s", s.ID, s.After)
		}
		if strings.HasPrefix(s.Before, "js, err := nc.JetStream()") && !strings.HasPrefix(s.After, "js, err := jetstream.New(nc)") {
			t.Errorf("root %s after text:\n%s\nwant it to start with js, err := jetstream.New(nc)", s.ID, s.After)
		}
	}
	applyAll(t, dir, plan, patterns...)
	if got := legacyResidue(t, dir, patterns...); len(got) > 0 {
		t.Errorf("legacy uses left: %v", got)
	}
	for _, pkg := range []string{"migrate/independent", "migrate/multifile"} {
		for path, text := range goFiles(t, dir, pkg) {
			if strings.Contains(text, "_ = js") || strings.Contains(text, "jsNew") {
				t.Errorf("%s keeps a placeholder or sibling:\n%s", path, text)
			}
		}
	}
}
