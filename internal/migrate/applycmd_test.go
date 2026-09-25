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
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// applyCmd runs `natsvet migrate apply args...` in dir.
func applyCmd(t *testing.T, dir string, args ...string) (int, string) {
	t.Helper()
	t.Chdir(dir)
	var out, errOut bytes.Buffer
	code := Main(append([]string{"apply"}, args...), &out, &errOut)
	return code, out.String() + errOut.String()
}

// answerModule writes natsvet-migrate.json answering component migrate for
// the module.
func answerModule(t *testing.T, dir string) {
	t.Helper()
	b, _ := json.Marshal(Decisions{Version: decisionsVersion, Answers: []Answer{{Pattern: patComponent, Scope: "module", Choice: "migrate"}}})
	if err := os.WriteFile(filepath.Join(dir, decisionsFile), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestApplyNextStep(t *testing.T) {
	pinnedJetStream(t)
	dir := copyModule(t, testdataDir, "migrate", "migrateext")
	patterns := []string{"./migrate/independent"}
	answerModule(t, dir)
	plan, err := buildPlan(options{dir: dir, patterns: patterns, tests: true})
	if err != nil {
		t.Fatal(err)
	}
	want := copyModule(t, testdataDir, "migrate", "migrateext")
	first := plan.Steps[0]
	if err := applyStep(want, first); err != nil {
		t.Fatal(err)
	}
	code, out := applyCmd(t, dir, patterns...)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	for _, w := range []string{first.ID, first.Kind, "of component " + first.Component, "migrate/independent/independent.go", "migrate/independent.Cleanup", "next:"} {
		if !strings.Contains(out, w) {
			t.Errorf("output lacks %q:\n%s", w, out)
		}
	}
	if got, w := goFiles(t, dir, "migrate/independent"), goFiles(t, want, "migrate/independent"); !mapsEqual(got, w) {
		t.Error("apply did not write exactly the plan's first step")
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestApplyComponentStopsAtGuided(t *testing.T) {
	pinnedJetStream(t)
	dir := copyModule(t, testdataDir, "migrate", "migrateext")
	patterns := []string{"./migrate/guided"}
	answerModule(t, dir)
	plan, err := buildPlan(options{dir: dir, patterns: patterns, tests: true})
	if err != nil {
		t.Fatal(err)
	}
	comp := plan.Components[0]
	var guided Step
	for _, st := range plan.Steps {
		if st.Kind == stepSite && !st.Machine {
			guided = st
		}
	}
	code, out := applyCmd(t, dir, append([]string{"-component", comp.ID}, patterns...)...)
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	for _, w := range []string{"add-handle", guided.ID, guided.Sites[0]} {
		if !strings.Contains(out, w) {
			t.Errorf("output lacks %q:\n%s", w, out)
		}
	}
	if errs := typeErrors(dir, patterns...); len(errs) > 0 {
		t.Errorf("the module does not type-check: %v", errs)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "migrate/guided/guided.go"))
	if !strings.Contains(string(b), "jsNew.AccountInfo(context.Background())") || !strings.Contains(string(b), "js.PullSubscribe") {
		t.Errorf("want the mechanical site migrated and the guided one untouched:\n%s", b)
	}
}

func TestApplyDryRun(t *testing.T) {
	pinnedJetStream(t)
	dir := copyModule(t, testdataDir, "migrate", "migrateext")
	answerModule(t, dir)
	before := goFiles(t, dir, "migrate")
	code, out := applyCmd(t, dir, "-dry-run", "./migrate/independent")
	if code != 0 {
		t.Fatalf("exit %d:\n%s", code, out)
	}
	for _, w := range []string{"--- a/migrate/independent/independent.go", "+++ b/migrate/independent/independent.go", "@@ ", "+\tjs, err := jetstream.New(nc)"} {
		if !strings.Contains(out, w) {
			t.Errorf("diff lacks %q:\n%s", w, out)
		}
	}
	if !mapsEqual(goFiles(t, dir, "migrate"), before) {
		t.Error("a dry run changed files")
	}
}

func TestApplyStepZero(t *testing.T) {
	pinnedJetStream(t)
	src := filepath.Join(testdataDir, "migrate-old")
	if _, err := os.Stat(src); err != nil {
		t.Skip(err)
	}
	dir := copyTree(t, src)
	if _, err := buildPlan(options{dir: dir, patterns: []string{"./..."}, tests: true}); err != nil {
		t.Skipf("the old nats.go is not available: %v", err)
	}
	answerModule(t, dir)
	before := goFiles(t, dir, ".")
	code, out := applyCmd(t, dir, "./...")
	if code != 3 || !strings.Contains(out, "go get github.com/nats-io/nats.go@"+tableVersion) {
		t.Errorf("exit %d, want 3 with the go get command:\n%s", code, out)
	}
	if !mapsEqual(goFiles(t, dir, "."), before) {
		t.Error("apply changed files before step 0")
	}
}

func TestApplyRefusesUnansweredComponent(t *testing.T) {
	pinnedJetStream(t)
	dir := copyModule(t, testdataDir, "migrate", "migrateext")
	before := goFiles(t, dir, "migrate")
	code, out := applyCmd(t, dir, "./migrate/independent")
	if code == 0 || !strings.Contains(out, "component") || !strings.Contains(out, decisionsFile) {
		t.Errorf("exit %d, want a refusal naming the pending component question:\n%s", code, out)
	}
	if !mapsEqual(goFiles(t, dir, "migrate"), before) {
		t.Error("a refused apply changed files")
	}
}

func TestApplyRollsBack(t *testing.T) {
	pinnedJetStream(t)
	dir := copyModule(t, testdataDir, "migrate", "migrateext")
	answerModule(t, dir)
	before := goFiles(t, dir, "migrate")
	applyHook = func(st *Step) { st.Edits[len(st.Edits)-1].New += " undefinedName" }
	t.Cleanup(func() { applyHook = nil })
	code, out := applyCmd(t, dir, "./migrate/independent")
	if code == 0 || !strings.Contains(out, "undefinedName") {
		t.Errorf("exit %d, want a failure naming the type error:\n%s", code, out)
	}
	if !mapsEqual(goFiles(t, dir, "migrate"), before) {
		t.Error("a step that does not compile was left in the files")
	}
}

func TestApplyNothingLeft(t *testing.T) {
	pinnedJetStream(t)
	dir := copyModule(t, testdataDir, "migrate", "migrateext")
	patterns := []string{"./migrate/legacyonly"}
	answerModule(t, dir)
	for range 10 {
		code, out := applyCmd(t, dir, patterns...)
		if code != 0 {
			t.Fatalf("exit %d:\n%s", code, out)
		}
		if strings.Contains(out, "nothing left to apply") {
			if !strings.Contains(out, "unmapped") {
				t.Errorf("output does not name the remaining unmapped site:\n%s", out)
			}
			return
		}
	}
	t.Fatal("apply never ran out of steps")
}

// applyUntilDone runs apply until it reports nothing left, and returns
// the step ids it applied.
func applyUntilDone(t *testing.T, dir string, patterns ...string) []string {
	t.Helper()
	var applied []string
	for range 20 {
		code, out := applyCmd(t, dir, patterns...)
		if code != 0 {
			t.Fatalf("exit %d:\n%s", code, out)
		}
		if strings.Contains(out, "nothing left to apply") {
			return applied
		}
		for _, l := range strings.Split(out, "\n") {
			if id, ok := strings.CutPrefix(l, "applied "); ok {
				applied = append(applied, strings.Fields(id)[0])
			}
		}
	}
	t.Fatal("apply never ran out of steps")
	return nil
}

// TestApplyGuidedEndToEnd migrates a component with a guided site with
// apply alone, the test migrating the guided site by hand in between, and
// reaches the files the test applier's chain reaches.
func TestApplyGuidedEndToEnd(t *testing.T) {
	pinnedJetStream(t)
	dir := copyModule(t, testdataDir, "migrate", "migrateext")
	patterns := []string{"./migrate/guided"}
	answerModule(t, dir)
	// apply changes the working directory: read the fixtures first.
	fixture, err := os.ReadFile(filepath.Join("testdata", "guided.fixture"))
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("testdata", "guided.final.go.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := applyUntilDone(t, dir, patterns...); len(got) != 2 {
		t.Errorf("before the hand edit apply applied %v, want add-handle and the mechanical site", got)
	}
	file := filepath.Join(dir, "migrate/guided/guided.go")
	writeGuided(t, file, fixture)
	if got := applyUntilDone(t, dir, patterns...); len(got) != 1 {
		t.Errorf("after the hand edit apply applied %v, want the finish step", got)
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, want) {
		t.Errorf("apply reached:\n%s\nwant (testdata/guided.final.go.txt):\n%s", b, want)
	}
}
