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
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// planDir plans patterns in the module at dir with the given answers in
// its natsvet-migrate.json.
func planDir(t *testing.T, dir string, answers []Answer, patterns ...string) (*Plan, error) {
	t.Helper()
	b, _ := json.Marshal(Decisions{Version: decisionsVersion, Answers: answers})
	if err := os.WriteFile(filepath.Join(dir, decisionsFile), b, 0o644); err != nil {
		t.Fatal(err)
	}
	return buildPlan(options{dir: dir, patterns: patterns, tests: true})
}

// insertBefore inserts text before the first occurrence of anchor in the
// file at path.
func insertBefore(t *testing.T, path, anchor, text string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i := strings.Index(s, anchor)
	if i < 0 {
		t.Fatalf("%s has no %q", path, anchor)
	}
	if err := os.WriteFile(path, []byte(s[:i]+text+s[i:]), 0o644); err != nil {
		t.Fatal(err)
	}
}

// subscribeChoice returns the subscribe-target choice of the site whose
// before text contains subject.
func subscribeChoice(t *testing.T, plan *Plan, subject string) string {
	t.Helper()
	s := siteWith(t, plan, "migrate/neighbors/neighbors.go", subject)
	d, ok := decisionOf(s, patSubscribeTarget)
	if !ok {
		t.Fatalf("site %s has no subscribe-target decision", s.ID)
	}
	return d.Choice
}

// TestIdsSurviveEdits answers a component and a site by id, edits the code
// around them, and checks that the answers still apply to the same code.
func TestIdsSurviveEdits(t *testing.T) {
	pinnedJetStream(t)
	dir := copyModule(t, testdataDir, "migrate", "migrateext")
	patterns := []string{"./migrate/neighbors"}
	plan, err := planDir(t, dir, nil, patterns...)
	if err != nil {
		t.Fatal(err)
	}
	legacy := ""
	for _, c := range plan.Components {
		if strings.Contains(strings.Join(c.Handles, " "), "legacy.go") {
			legacy = c.ID
		}
	}
	if legacy == "" {
		t.Fatal("no component in legacy.go")
	}
	old := siteWith(t, plan, "migrate/neighbors/neighbors.go", "orders.old").ID
	answers := []Answer{
		{Pattern: patComponent, Scope: "module", Choice: "migrate"},
		{Pattern: patComponent, Scope: "component:" + legacy, Choice: "skip"},
		{Pattern: patSubscribeTarget, Scope: "module", Choice: "pull"},
		{Pattern: patSubscribeTarget, Scope: "site:" + old, Choice: "push"},
	}
	check := func(t *testing.T, plan *Plan) {
		t.Helper()
		if len(plan.StaleAnswers) > 0 {
			t.Errorf("stale answers: %+v", plan.StaleAnswers)
		}
		skipped := false
		for _, c := range plan.Components {
			if strings.Contains(strings.Join(c.Handles, " "), "legacy.go") {
				skipped = c.Skipped
			}
		}
		if !skipped {
			t.Error("the component in legacy.go is not skipped")
		}
		if got := subscribeChoice(t, plan, "orders.old"); got != "push" {
			t.Errorf("orders.old: subscribe-target %q, want push", got)
		}
		if got := subscribeChoice(t, plan, "orders.new"); got != "pull" {
			t.Errorf("orders.new: subscribe-target %q, want pull", got)
		}
	}
	plan, err = planDir(t, dir, answers, patterns...)
	if err != nil {
		t.Fatal(err)
	}
	check(t, plan)
	insertBefore(t, filepath.Join(dir, "migrate/neighbors/legacy.go"), "func Legacy", "func helper() int { return 1 }\n\n")
	insertBefore(t, filepath.Join(dir, "migrate/neighbors/neighbors.go"), "\t_, errNew", "\t// Both subscriptions share the handler.\n")
	plan, err = planDir(t, dir, answers, patterns...)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("edits above", func(t *testing.T) { check(t, plan) })
	t.Run("renamed function", func(t *testing.T) {
		path := filepath.Join(dir, "migrate/neighbors/legacy.go")
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(strings.Replace(string(b), "func Legacy", "func Start", 1)), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err = planDir(t, dir, answers, patterns...)
		if err == nil || !strings.Contains(err.Error(), "component:"+legacy) {
			t.Errorf("a stale skip answer: err = %v, want an error naming component:%s", err, legacy)
		}
	})
}

func TestDecisionsVersion1(t *testing.T) {
	pinnedJetStream(t)
	dir := copyModule(t, testdataDir, "migrate", "migrateext")
	if err := os.WriteFile(filepath.Join(dir, decisionsFile), []byte(`{"version": 1, "answers": []}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := buildPlan(options{dir: dir, patterns: []string{"./migrate/neighbors"}, tests: true})
	for _, want := range []string{"version 2", "component:<pkg>.<Func>#<handle>", "site:<pkg>.<Func>#<Symbol>@<hash>"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("a version 1 file: err = %v, want it to name %q", err, want)
		}
	}
}

// TestStaleSiteAnswer checks that a site answer whose site is gone is
// reported, and the plan is still produced.
func TestStaleSiteAnswer(t *testing.T) {
	pinnedJetStream(t)
	dir := copyModule(t, testdataDir, "migrate", "migrateext")
	patterns := []string{"./migrate/neighbors"}
	plan, err := planDir(t, dir, nil, patterns...)
	if err != nil {
		t.Fatal(err)
	}
	old := siteWith(t, plan, "migrate/neighbors/neighbors.go", "orders.old").ID
	path := filepath.Join(dir, "migrate/neighbors/neighbors.go")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i := strings.Index(s, "\t_, errOld := js.Subscribe(")
	j := i + strings.Index(s[i:], "\n")
	if err := os.WriteFile(path, []byte(s[:i]+"\tvar errOld error"+s[j:]), 0o644); err != nil {
		t.Fatal(err)
	}
	answer := Answer{Pattern: patSubscribeTarget, Scope: "site:" + old, Choice: "push"}
	plan, err = planDir(t, dir, []Answer{answer}, patterns...)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(plan.StaleAnswers, answer) {
		t.Errorf("stale answers %+v, want %+v", plan.StaleAnswers, answer)
	}
}
