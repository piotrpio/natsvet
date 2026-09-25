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
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// planFor plans testdata packages with the given answers.
func planFor(t *testing.T, answers []Answer, patterns ...string) *Plan {
	t.Helper()
	pinnedJetStream(t)
	plan, err := planErr(t, answers, patterns...)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func planErr(t *testing.T, answers []Answer, patterns ...string) (*Plan, error) {
	t.Helper()
	path := ""
	if answers != nil {
		path = filepath.Join(t.TempDir(), decisionsFile)
		b, _ := json.Marshal(Decisions{Version: 1, Answers: answers})
		if err := os.WriteFile(path, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return buildPlan(options{dir: testdataDir, patterns: patterns, tests: true, decisions: path})
}

// siteWith returns the site of file whose before text contains substr.
func siteWith(t *testing.T, plan *Plan, file, substr string) Site {
	t.Helper()
	for _, s := range plan.Sites {
		if s.Position.File == file && strings.Contains(s.Before, substr) {
			return s
		}
	}
	t.Fatalf("no site in %s shows %q", file, substr)
	return Site{}
}

func decisionOf(s Site, pattern string) (SiteDecision, bool) {
	for _, d := range s.Decisions {
		if d.Pattern == pattern {
			return d, true
		}
	}
	return SiteDecision{}, false
}

const (
	scenarios = "migrate/scenarios/scenarios.go"
	appFile   = "migrate/app/app.go"
	subFile   = "migrate/app/subscribe.go"
)

func TestContextRule(t *testing.T) {
	plan := planFor(t, nil, "./migrate/scenarios")
	for _, tc := range []struct{ name, before, want string }{
		{"context in scope", "js.AddStream(&cfg)", "CreateStream(ctx, cfg)"},
		{"context from nats.Context", "nats.Context(r.ctx)", "CreateStream(r.ctx, cfg)"},
		{"no context anywhere", `js.DeleteStream("S")`, `DeleteStream(context.Background(), "S")`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := siteWith(t, plan, scenarios, tc.before)
			if s.Class != classMechanical || !strings.Contains(s.After, tc.want) {
				t.Errorf("%s: class %s, after %q; want mechanical with %q", tc.before, s.Class, s.After, tc.want)
			}
		})
	}
}

func TestMechanicalReplacements(t *testing.T) {
	plan := planFor(t, nil, "./migrate/...")
	for _, tc := range []struct {
		name, file, before string
		want               []string
		note               string
	}{
		{"consumer creation", scenarios, `Heartbeat: time.Second`,
			[]string{`jsNew.CreatePushConsumer(ctx, "ORDERS", jetstream.ConsumerConfig{Durable: "w", DeliverSubject: "w.deliver", IdleHeartbeat: time.Second, AckPolicy: jetstream.AckNonePolicy})`}, "create-only"},
		{"purge through a stream handle", appFile, "PurgeStream",
			[]string{`.Stream(context.Background(), "ORDERS")`, "stream.Purge(context.Background())"}, "STREAM.INFO"},
		{"options folded into a config", scenarios, "nats.BindStream",
			[]string{`Durable: "w"`, "DeliverPolicy: jetstream.DeliverNewPolicy", "MaxAckPending: 100", `FilterSubject: "orders.new"`, `CreateOrUpdateConsumer(context.Background(), "ORDERS"`}, ""},
		{"runtime stream lookup", scenarios, `js.PullSubscribe("orders.new", "w")`,
			[]string{`StreamNameBySubject(context.Background(), "orders.new")`}, ""},
		{"plain call", scenarios, "StreamNameBySubject", []string{`jsNew.StreamNameBySubject(context.Background(), "orders.new")`}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := siteWith(t, plan, tc.file, tc.before)
			if s.Class != classMechanical {
				t.Fatalf("class %s (%v), want mechanical", s.Class, s.Facts)
			}
			for _, w := range tc.want {
				if !strings.Contains(s.After, w) {
					t.Errorf("after %q lacks %q", s.After, w)
				}
			}
			if tc.note != "" && !slices.ContainsFunc(s.Notes, func(n string) bool { return strings.Contains(n, tc.note) }) {
				t.Errorf("notes %v lack %q", s.Notes, tc.note)
			}
		})
	}
}

// parses reports whether text is Go: statements, an expression,
// declarations, struct fields or parameters.
func parses(text string) bool {
	fset := token.NewFileSet()
	for _, src := range []string{
		"package p\nfunc _() {\n" + text + "\n}",
		"package p\n" + text,
		"package p\nvar _ = " + text,
		"package p\ntype _ struct {\n" + text + "\n}",
		"package p\nfunc _(" + text + ")",
	} {
		if _, err := parser.ParseFile(fset, "", src, 0); err == nil {
			return true
		}
	}
	return false
}

func TestReplacementsParse(t *testing.T) {
	plan := planWithDefaults(t, copyModule(t, testdataDir, "migrate", "migrateext"), []string{"./migrate/..."})
	for _, s := range plan.Sites {
		if s.After != "" && !parses(s.After) {
			t.Errorf("%s: after does not parse:\n%s", s.ID, s.After)
		}
		for _, d := range s.Decisions {
			for _, o := range d.Options {
				if o.After != "" && !strings.Contains(o.After, ": ") && !parses(o.After) {
					t.Errorf("%s: %s=%s does not parse:\n%s", s.ID, d.Pattern, o.ID, o.After)
				}
			}
		}
	}
}

func TestGuidedAndUnmapped(t *testing.T) {
	plan := planFor(t, nil, "./migrate/...")
	fetch := siteWith(t, plan, subFile, `PullSubscribe("orders.new", "batch")`)
	if fetch.Class != classGuided {
		t.Fatalf("Fetch loop: class %s, want guided", fetch.Class)
	}
	for _, w := range []string{"batch.Messages()", "batch.Error()", "jetstream.FetchMaxWait(time.Second)"} {
		if !strings.Contains(fetch.Template, w) {
			t.Errorf("Fetch template lacks %q:\n%s", w, fetch.Template)
		}
	}
	if !slices.ContainsFunc(fetch.Facts, func(f string) bool {
		return strings.Contains(f, "ErrTimeout") && strings.Contains(f, "empty pull is not an error")
	}) {
		t.Errorf("Fetch facts %v do not explain the dropped ErrTimeout branch", fetch.Facts)
	}
	timed := siteWith(t, plan, scenarios, "nats.MaxWait(time.Second)")
	if timed.Class != classGuided || !slices.ContainsFunc(timed.Facts, func(f string) bool { return strings.Contains(f, "context.WithTimeout") }) {
		t.Errorf("per-call MaxWait: class %s facts %v, want guided with context.WithTimeout", timed.Class, timed.Facts)
	}
	legacy := siteWith(t, plan, "migrate/legacyonly/legacyonly.go", "UseLegacyDurableConsumers")
	if legacy.Class != classUnmapped || len(legacy.Notes) == 0 || legacy.After != "" {
		t.Errorf("legacy-only option: class %s notes %v after %q, want unmapped with a reason and no replacement", legacy.Class, legacy.Notes, legacy.After)
	}
	if plan.Counts.Unmapped == 0 {
		t.Error("unmapped sites are not counted")
	}
}

func TestSubscribeDecisions(t *testing.T) {
	plan := planFor(t, nil, "./migrate/...")
	run := siteWith(t, plan, subFile, `"orders.new", func`)
	if d, ok := decisionOf(run, patSubscribeTarget); !ok || d.Default != "pull" {
		t.Errorf("unbound callback: subscribe-target %+v, want default pull", d)
	}
	if d, ok := decisionOf(run, patAck); !ok || d.Default != "after-handler" {
		t.Errorf("unbound callback: ack %+v, want default after-handler", d)
	}
	if run.Class != classDecision || run.After != "" {
		t.Errorf("pending subscription: class %s after %q, want decision without a replacement", run.Class, run.After)
	}
	queue := siteWith(t, plan, subFile, `nats.Bind("ORDERS", "queue")`)
	if d, ok := decisionOf(queue, patSubscribeTarget); !ok || d.Default != "push" || !strings.Contains(d.Options[1].After, `PushConsumer(ctx, "ORDERS", "queue")`) {
		t.Errorf("bound push: subscribe-target %+v, want default push through PushConsumer", d)
	}
	if _, ok := decisionOf(queue, patAck); ok {
		t.Error("ManualAck subscription has an ack decision")
	}
	bound := siteWith(t, plan, scenarios, "nats.BindStream")
	if _, ok := decisionOf(bound, patSubscribeTarget); ok {
		t.Error("bound pull subscription has a subscribe-target decision")
	}
	for _, f := range plan.FollowUps {
		if f.Site == bound.ID {
			t.Errorf("bound pull subscription has a follow-up %+v", f)
		}
	}
	// A push-only option under push becomes a config field.
	limited := siteWith(t, plan, subFile, "nats.RateLimit(1024)")
	pushed := planFor(t, []Answer{{Pattern: patSubscribeTarget, Scope: "site:" + limited.ID, Choice: "push"}}, "./migrate/...")
	limited = siteWith(t, pushed, subFile, "nats.RateLimit(1024)")
	if _, ok := decisionOf(limited, patPushOnly); ok {
		t.Error("push-only option under push raises push-only-option")
	}
	if !slices.ContainsFunc(limited.Decisions[1].Options, func(o Alternative) bool { return strings.Contains(o.After, "RateLimit: 1024") }) {
		t.Errorf("push subscription does not carry RateLimit: %+v", limited.Decisions)
	}
	pulled := planFor(t, []Answer{{Pattern: patSubscribeTarget, Scope: "site:" + limited.ID, Choice: "pull"}}, "./migrate/...")
	limited = siteWith(t, pulled, subFile, "nats.RateLimit(1024)")
	if d, ok := decisionOf(limited, patPushOnly); !ok || d.Default != "drop" {
		t.Errorf("push-only option under pull: %+v, want push-only-option defaulting to drop", d)
	}
}

func TestDurableNoteAndFollowUps(t *testing.T) {
	plan := planFor(t, nil, "./migrate/...")
	hasDurable := func(s Site) bool {
		return slices.ContainsFunc(s.Notes, func(n string) bool { return strings.Contains(n, "durable") && strings.Contains(n, "keeps it") })
	}
	run := siteWith(t, plan, subFile, `nats.Durable("workers")`)
	if !hasDurable(run) {
		t.Errorf("explicit durable: notes %v lack the durable note", run.Notes)
	}
	named := siteWith(t, plan, subFile, `nats.ConsumerName("limited")`)
	if hasDurable(named) {
		t.Errorf("named ephemeral carries the durable note: %v", named.Notes)
	}
	for _, s := range plan.Sites {
		if strings.Contains(s.Before, "Subscribe") {
			for _, d := range s.Decisions {
				for _, o := range d.Options {
					if strings.Contains(o.After, "DeleteConsumer") {
						t.Errorf("%s proposes DeleteConsumer", s.ID)
					}
				}
			}
		}
	}
	followUp := func(id string) FollowUp {
		for _, f := range plan.FollowUps {
			if f.Site == id && f.Kind == "stream-name" {
				return f
			}
		}
		t.Fatalf("no stream-name follow-up for %s", id)
		return FollowUp{}
	}
	if f := followUp(run.ID); f.Candidate == nil || f.Candidate.File != appFile || !strings.Contains(f.Summary, `"ORDERS"`) {
		t.Errorf("covered subject: follow-up %+v, want the ORDERS stream created in app.go", f)
	}
	billing := siteWith(t, plan, scenarios, `"billing.new"`)
	if f := followUp(billing.ID); f.Candidate != nil {
		t.Errorf("uncovered subject: follow-up %+v names a candidate", f)
	}
}

func TestComponentSteps(t *testing.T) {
	plan := planFor(t, nil, "./migrate/...")
	comp := func(file string) Component {
		for _, c := range plan.Components {
			if strings.HasPrefix(c.ID, file+":") {
				return c
			}
		}
		t.Fatalf("no component in %s", file)
		return Component{}
	}
	steps := make(map[string]Step)
	for _, st := range plan.Steps {
		steps[st.ID] = st
	}
	if c := comp("migrate/independent/independent.go"); !c.OneCommit {
		t.Errorf("package-local mechanical component %s is not marked one-commit", c.ID)
	}
	app := comp(appFile)
	if app.OneCommit {
		t.Error("the service component with guided sites is marked one-commit")
	}
	fetch := siteWith(t, plan, subFile, `PullSubscribe("orders.new", "batch")`)
	for _, id := range app.Steps {
		st := steps[id]
		if (st.Kind == stepRemove || st.Kind == stepRename) && (st.Machine || !slices.Contains(st.WaitsOn, fetch.ID)) {
			t.Errorf("%s step %s: machine %v, waits on %v; want waiting on the Fetch site %s", st.Kind, st.ID, st.Machine, st.WaitsOn, fetch.ID)
		}
	}
	b := comp("migrate/boundary/boundary.go")
	if len(b.Blocked) == 0 {
		t.Error("the component passing its handle outside is not blocked")
	}
	for _, id := range b.Steps {
		if k := steps[id].Kind; k == stepRemove || k == stepRename {
			t.Errorf("blocked component has a %s step", k)
		}
	}
	for _, st := range plan.Steps {
		if st.Kind == stepGoGet {
			t.Errorf("a module on the table's nats.go has a step 0: %+v", st)
		}
	}
}

func TestStepZero(t *testing.T) {
	pinnedJetStream(t)
	dir := filepath.Join(testdataDir, "migrate-old")
	if _, err := os.Stat(dir); err != nil {
		t.Skip(err)
	}
	plan, err := buildPlan(options{dir: dir, patterns: []string{"./..."}, tests: true})
	if err != nil {
		t.Skipf("the old nats.go is not available: %v", err)
	}
	if len(plan.Steps) == 0 || plan.Steps[0].Kind != stepGoGet || plan.Steps[0].Command != "go get github.com/nats-io/nats.go@latest" {
		t.Errorf("first step %+v, want go get github.com/nats-io/nats.go@latest", plan.Steps[0])
	}
	if plan.ModuleNatsVersion != "v1.31.0" {
		t.Errorf("module nats.go %q, want v1.31.0", plan.ModuleNatsVersion)
	}
}

func TestDecisionAnswers(t *testing.T) {
	base := planFor(t, nil, "./migrate/...")
	run := siteWith(t, base, subFile, `nats.Durable("workers")`)
	t.Run("one answer covers many", func(t *testing.T) {
		plan := planFor(t, []Answer{{Pattern: patSubscribeTarget, Scope: "module", Choice: "pull"}}, "./migrate/...")
		for _, p := range plan.Pending {
			if p.Pattern == patSubscribeTarget {
				t.Errorf("subscribe-target still pending: %+v", p)
			}
		}
		for _, s := range plan.Sites {
			if d, ok := decisionOf(s, patSubscribeTarget); ok && d.Choice != "pull" {
				t.Errorf("%s: subscribe-target choice %q, want pull", s.ID, d.Choice)
			}
		}
	})
	t.Run("site override", func(t *testing.T) {
		plan := planFor(t, []Answer{
			{Pattern: patSubscribeTarget, Scope: "module", Choice: "pull"},
			{Pattern: patSubscribeTarget, Scope: "site:" + run.ID, Choice: "push"},
			{Pattern: patAck, Scope: "module", Choice: "after-handler"},
		}, "./migrate/...")
		if s := siteWith(t, plan, subFile, `nats.Durable("workers")`); !strings.Contains(s.After, "CreateOrUpdatePushConsumer") {
			t.Errorf("overridden site: after %q, want a push consumer", s.After)
		}
		if s := siteWith(t, plan, scenarios, `"billing.new"`); !strings.Contains(s.After, "CreateOrUpdateConsumer(") || strings.Contains(s.After, "Push") {
			t.Errorf("other site: after %q, want a pull consumer", s.After)
		}
	})
	t.Run("skipped component", func(t *testing.T) {
		var id string
		for _, c := range base.Components {
			if strings.HasPrefix(c.ID, "migrate/legacytests/") {
				id = c.ID
			}
		}
		plan := planFor(t, []Answer{{Pattern: patComponent, Scope: "component:" + id, Choice: "skip"}}, "./migrate/...")
		skipped := 0
		for _, s := range plan.Sites {
			if s.Component == id {
				if !s.Skipped {
					t.Errorf("%s is not skipped", s.ID)
				}
				skipped++
			}
		}
		if skipped == 0 || plan.Counts.Skipped != skipped {
			t.Errorf("skipped count %d, want %d", plan.Counts.Skipped, skipped)
		}
		for _, st := range plan.Steps {
			if st.Component == id {
				t.Errorf("skipped component has step %s", st.ID)
			}
		}
	})
	t.Run("unknown option", func(t *testing.T) {
		_, err := planErr(t, []Answer{{Pattern: patSubscribeTarget, Scope: "module", Choice: "poll"}}, "./migrate/...")
		if err == nil || !strings.Contains(err.Error(), patSubscribeTarget) || !strings.Contains(err.Error(), "poll") {
			t.Errorf("err = %v, want one naming subscribe-target and poll", err)
		}
	})
	t.Run("unknown pattern and scope", func(t *testing.T) {
		if _, err := planErr(t, []Answer{{Pattern: "retry", Scope: "module", Choice: "x"}}, "./migrate/..."); err == nil {
			t.Error("unknown pattern accepted")
		}
		if _, err := planErr(t, []Answer{{Pattern: patAck, Scope: "package:app", Choice: "none"}}, "./migrate/..."); err == nil {
			t.Error("unknown scope accepted")
		}
	})
	t.Run("stale answer", func(t *testing.T) {
		plan := planFor(t, []Answer{{Pattern: patAck, Scope: "site:migrate/app/gone.go:1:1", Choice: "none"}}, "./migrate/...")
		if len(plan.StaleAnswers) != 1 {
			t.Errorf("stale answers %v, want the one naming gone.go", plan.StaleAnswers)
		}
	})
	t.Run("deterministic", func(t *testing.T) {
		var outs [2]bytes.Buffer
		for i := range 2 {
			if err := writeJSON(&outs[i], planFor(t, nil, "./migrate/...")); err != nil {
				t.Fatal(err)
			}
		}
		if !bytes.Equal(outs[0].Bytes(), outs[1].Bytes()) {
			t.Error("two plans of the same packages differ")
		}
	})
}

func TestPlanCommandScenarios(t *testing.T) {
	t.Run("no legacy use", func(t *testing.T) {
		plan := planFor(t, nil, "./migrate/core")
		if len(plan.Sites) != 0 || len(plan.Components) != 0 {
			t.Errorf("core-only package: %d sites, %d components", len(plan.Sites), len(plan.Components))
		}
	})
	t.Run("type errors", func(t *testing.T) {
		pinnedJetStream(t)
		dir := copyModule(t, testdataDir, "migrate/core")
		if err := os.WriteFile(filepath.Join(dir, "migrate/core/broken.go"), []byte("package core\n\nvar _ int = \"x\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		var out, errOut bytes.Buffer
		wd, _ := os.Getwd()
		if err := os.Chdir(dir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(wd)
		if code := Main([]string{"plan", "./migrate/core"}, &out, &errOut); code == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), "broken.go") {
			t.Errorf("exit %d, stdout %q, stderr %q; want a failure naming broken.go and no plan", code, out.String(), errOut.String())
		}
	})
}

func TestEditsDoNotOverlap(t *testing.T) {
	plan := planFor(t, nil, "./migrate/...")
	for _, st := range plan.Steps {
		byFile := make(map[string][]Edit)
		for _, e := range st.Edits {
			byFile[e.File] = append(byFile[e.File], e)
		}
		for f, es := range byFile {
			slices.SortFunc(es, func(a, b Edit) int { return a.Start - b.Start })
			for i := 1; i < len(es); i++ {
				if es[i].Start < es[i-1].End {
					t.Errorf("step %s: edits in %s overlap: %+v and %+v", st.ID, f, es[i-1], es[i])
				}
			}
		}
	}
}

func TestMarkdownMirrorsJSON(t *testing.T) {
	plan := planFor(t, nil, "./migrate/...")
	var md bytes.Buffer
	if err := writeMarkdown(&md, plan); err != nil {
		t.Fatal(err)
	}
	text := md.String()
	for _, s := range plan.Sites {
		if !strings.Contains(text, "Site "+s.ID+":") {
			t.Errorf("markdown lacks site %s", s.ID)
		}
	}
	for _, st := range plan.Steps {
		if !strings.Contains(text, "Step "+st.ID+" ") && !strings.Contains(text, "Step "+st.ID+":") {
			t.Errorf("markdown lacks step %s", st.ID)
		}
	}
	for _, p := range plan.Pending {
		if !strings.Contains(text, "**"+p.Pattern+"** ("+p.Scope+",") {
			t.Errorf("markdown lacks pending %s at %s", p.Pattern, p.Scope)
		}
	}
	for _, f := range plan.FollowUps {
		if !strings.Contains(text, "- "+f.Site+" ("+f.Kind+"): ") {
			t.Errorf("markdown lacks follow-up %s %s", f.Site, f.Kind)
		}
	}
}

func TestSkillVersion(t *testing.T) {
	m := regexp.MustCompile("`schema_version: (\\d+)`").FindStringSubmatch(skill)
	if m == nil {
		t.Fatal("the skill names no schema version")
	}
	if m[1] != strconv.Itoa(schemaVersion) {
		t.Errorf("the skill understands schema version %s, plans carry %d", m[1], schemaVersion)
	}
	var out, errOut bytes.Buffer
	if code := Main([]string{"skill"}, &out, &errOut); code != 0 || out.String() != skill {
		t.Errorf("migrate skill: exit %d, printed %d bytes of %d", code, out.Len(), len(skill))
	}
	plan := planFor(t, nil, "./migrate/core")
	if plan.SchemaVersion != schemaVersion || !strings.Contains(plan.Skill, "natsvet migrate skill") {
		t.Errorf("plan header: version %d, skill %q", plan.SchemaVersion, plan.Skill)
	}
}

// TestPlanDeterministic plans a tree whose facts come from several uses and
// several values repeatedly: the plan must not depend on map order, and
// must name module files by their relative paths.
func TestPlanDeterministic(t *testing.T) {
	var first []byte
	for range 10 {
		var b bytes.Buffer
		if err := writeJSON(&b, planFor(t, nil, "./migrate/order")); err != nil {
			t.Fatal(err)
		}
		if first == nil {
			first = b.Bytes()
		} else if !bytes.Equal(b.Bytes(), first) {
			t.Fatalf("two plans of the same tree differ:\n%s\n---\n%s", first, b.Bytes())
		}
	}
	abs, err := filepath.Abs(testdataDir)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(first, []byte(abs)) {
		t.Errorf("the plan names absolute paths under %s:\n%s", abs, first)
	}
}

func TestHelperHandle(t *testing.T) {
	plan := planFor(t, nil, "./migrate/scenarios")
	get := siteWith(t, plan, "migrate/scenarios/corpus.go", `kv.Get("a")`)
	if get.Class != classGuided || !slices.ContainsFunc(get.Facts, func(f string) bool { return strings.Contains(f, "cannot be threaded") }) {
		t.Fatalf("Get on a helper's handle: class %s facts %v, want guided because the handle cannot be threaded", get.Class, get.Facts)
	}
	for _, st := range plan.Steps {
		if st.Component == get.Component && (st.Kind == stepRemove || st.Kind == stepRename) && (st.Machine || !slices.Contains(st.WaitsOn, get.ID)) {
			t.Errorf("%s step %s: machine %v, waits on %v; want waiting on %s", st.Kind, st.ID, st.Machine, st.WaitsOn, get.ID)
		}
	}
}

// An ack answer of none replaces the ack policy an option set, so the
// config literal names AckPolicy once.
func TestAckNoneOverridesAckOption(t *testing.T) {
	base := planFor(t, nil, "./migrate/scenarios")
	all := siteWith(t, base, scenarios, "nats.AckAll()")
	for _, tc := range []struct{ choice, want string }{
		{"after-handler", "AckPolicy: jetstream.AckAllPolicy"},
		{"none", "AckPolicy: jetstream.AckNonePolicy"},
	} {
		t.Run(tc.choice, func(t *testing.T) {
			plan := planFor(t, []Answer{
				{Pattern: patSubscribeTarget, Scope: "site:" + all.ID, Choice: "pull"},
				{Pattern: patAck, Scope: "site:" + all.ID, Choice: tc.choice},
			}, "./migrate/scenarios")
			s := siteWith(t, plan, scenarios, "nats.AckAll()")
			if n := strings.Count(s.After, "AckPolicy:"); n != 1 || !strings.Contains(s.After, tc.want) || !parses(s.After) {
				t.Errorf("after names AckPolicy %d times, want once as %q:\n%s", n, tc.want, s.After)
			}
		})
	}
}
