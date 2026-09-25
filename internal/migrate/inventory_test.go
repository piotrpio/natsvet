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
	"go/token"
	"slices"
	"testing"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/checker"
	"golang.org/x/tools/go/packages"

	"github.com/piotrpio/natsvet/analyzers/legacyjs"
)

// loadTestdata loads packages of the testdata module, skipping the test
// when the pinned nats.go is not available.
func loadTestdata(t *testing.T, patterns ...string) *program {
	t.Helper()
	pinnedJetStream(t)
	prog, err := load(testdataDir, patterns, true)
	if err != nil {
		t.Fatal(err)
	}
	return prog
}

// usePositions returns the positions of every legacy use in sites.
func usePositions(prog *program, sites []*site) []string {
	var out []string
	for _, s := range sites {
		for _, u := range s.uses {
			if !u.value {
				out = append(out, prog.fset.Position(u.id.Pos()).String())
			}
		}
	}
	slices.Sort(out)
	return out
}

func TestInventoryAgreesWithLegacyjs(t *testing.T) {
	prog := loadTestdata(t, "./migrate/...")
	got := usePositions(prog, inventory(prog))

	cfg := &packages.Config{Mode: packages.LoadAllSyntax, Dir: testdataDir, Tests: true, Fset: token.NewFileSet()}
	pkgs, err := packages.Load(cfg, "./migrate/...")
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
	var want []string
	for act := range graph.All() {
		if act.Analyzer != legacyjs.Analyzer {
			continue
		}
		for _, d := range act.Diagnostics {
			want = append(want, cfg.Fset.Position(d.Pos).String())
		}
	}
	slices.Sort(want)
	want = slices.Compact(want)
	if len(want) == 0 {
		t.Fatal("legacyjs reported nothing over testdata/migrate")
	}
	if !slices.Equal(got, want) {
		t.Errorf("inventory found %d legacy uses, legacyjs %d\ninventory: %v\nlegacyjs:  %v", len(got), len(want), got, want)
	}
}

func TestInventoryGroupsSubscribeCall(t *testing.T) {
	prog := loadTestdata(t, "./migrate/app")
	for _, s := range inventory(prog) {
		if s.sym != "JetStream.Subscribe" {
			continue
		}
		var syms []string
		for _, u := range s.uses {
			syms = append(syms, u.sym)
		}
		if slices.Contains(syms, "Durable") {
			want := []string{"JetStream.Subscribe", "Durable", "DeliverNew"}
			if !slices.Equal(syms, want) {
				t.Errorf("subscribe site uses = %v, want %v", syms, want)
			}
			return
		}
	}
	t.Fatal("no Subscribe site with a Durable option found")
}

func TestInventoryIgnoresCoreNATS(t *testing.T) {
	prog := loadTestdata(t, "./migrate/core")
	if sites := inventory(prog); len(sites) != 0 {
		t.Errorf("core-only package has %d sites", len(sites))
	}
}
