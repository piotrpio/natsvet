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
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

const testdataDir = "../../testdata"

// pinnedJetStream loads the jetstream package of the nats.go the testdata
// module pins, and that module's directory and version.
func pinnedJetStream(t *testing.T) (pkg *types.Package, modDir, version string) {
	t.Helper()
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedTypes | packages.NeedModule, Dir: testdataDir}
	pkgs, err := packages.Load(cfg, "github.com/nats-io/nats.go/jetstream")
	if err != nil || len(pkgs) != 1 || len(pkgs[0].Errors) > 0 || pkgs[0].Module == nil {
		t.Skipf("pinned nats.go not available (run `make testdata-deps`): %v", err)
	}
	return pkgs[0].Types, pkgs[0].Module.Dir, pkgs[0].Module.Version
}

func TestTableCoversLegacySymbols(t *testing.T) {
	legacy := natsapi.LegacySymbols()
	for _, sym := range legacy {
		if _, ok := table[sym]; !ok {
			t.Errorf("legacy symbol %s has no mapping entry", sym)
		}
	}
	for sym := range table {
		if _, found := slices.BinarySearch(legacy, sym); !found {
			t.Errorf("mapping entry %s is not a legacy symbol", sym)
		}
	}
}

func TestTableTargetsExist(t *testing.T) {
	pkg, _, _ := pinnedJetStream(t)
	for sym, e := range table {
		if e.Target != "" && !resolves(pkg, e.Target) {
			t.Errorf("%s: target jetstream.%s does not exist", sym, e.Target)
		}
		for _, f := range e.Fields {
			if !resolves(pkg, "ConsumerConfig."+f.Name) {
				t.Errorf("%s: jetstream.ConsumerConfig has no field %s", sym, f.Name)
			}
		}
		if e.Kind == Unmapped && e.Note == "" {
			t.Errorf("%s: unmapped entries need a reason", sym)
		}
	}
}

func TestTableVersion(t *testing.T) {
	_, _, version := pinnedJetStream(t)
	if tableVersion != version {
		t.Errorf("table verified against nats.go %s, testdata pins %s", tableVersion, version)
	}
}

// resolves reports whether target ("Name" or "Type.Member") names an
// exported object, field or method of pkg.
func resolves(pkg *types.Package, target string) bool {
	typeName, member, hasMember := strings.Cut(target, ".")
	obj := pkg.Scope().Lookup(typeName)
	if obj == nil || !obj.Exported() {
		return false
	}
	if !hasMember {
		return true
	}
	found, _, _ := types.LookupFieldOrMethod(obj.Type(), true, pkg, member)
	return found != nil
}

var (
	guideLegacyRe = regexp.MustCompile("^`(nats|js|msg)\\.([A-Za-z]+)")
	guideNewRe    = regexp.MustCompile(`jetstream\.([A-Za-z]+)|\.([A-Za-z]+)\(|\b([A-Z][A-Za-z]*):`)
)

// guideRow is one legacy-to-new row of MIGRATION.md: the legacy table keys
// it can name and the identifiers its "new" column mentions.
type guideRow struct {
	line  int
	keys  []string
	names []string
}

func guideRows(t *testing.T, path string) []guideRow {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var rows []guideRow
	for i, line := range strings.Split(string(data), "\n") {
		cells := strings.Split(line, "|")
		if len(cells) < 4 || strings.TrimSpace(cells[0]) != "" {
			continue
		}
		legacy, repl := strings.TrimSpace(cells[1]), strings.TrimSpace(cells[2])
		m := guideLegacyRe.FindStringSubmatch(legacy)
		if m == nil {
			continue
		}
		row := guideRow{line: i + 1}
		switch m[1] {
		case "nats":
			row.keys = []string{m[2]}
		case "msg":
			row.keys = []string{"Msg." + m[2]}
		case "js":
			for _, recv := range []string{"JetStreamManager", "JetStream", "KeyValueManager", "ObjectStoreManager"} {
				row.keys = append(row.keys, recv+"."+m[2])
			}
		}
		if repl == "Unchanged" {
			row.names = []string{m[2]}
		}
		for _, n := range guideNewRe.FindAllStringSubmatch(repl, -1) {
			for _, g := range n[1:] {
				if g != "" {
					row.names = append(row.names, g)
				}
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func TestTableAgreesWithMigrationGuide(t *testing.T) {
	_, modDir, _ := pinnedJetStream(t)
	rows := guideRows(t, filepath.Join(modDir, "jetstream", "MIGRATION.md"))
	if len(rows) == 0 {
		t.Fatal("parsed no mapping rows from MIGRATION.md; the parser no longer matches its tables")
	}
	checked := 0
	for _, row := range rows {
		if len(row.names) == 0 {
			continue
		}
		var e Entry
		var key string
		for _, k := range row.keys {
			if found, ok := table[k]; ok {
				e, key = found, k
				break
			}
		}
		if key == "" {
			continue
		}
		checked++
		if e.Kind == Unmapped {
			t.Errorf("MIGRATION.md:%d maps %s to %v, the table says unmapped: %s", row.line, key, row.names, e.Note)
			continue
		}
		if !slices.ContainsFunc(names(e), func(n string) bool { return slices.Contains(row.names, n) }) {
			t.Errorf("MIGRATION.md:%d maps %s to %v, the table to %v", row.line, key, row.names, names(e))
		}
	}
	if checked == 0 {
		t.Fatal("no MIGRATION.md row matched a table entry")
	}
	t.Logf("checked %d of %d MIGRATION.md rows", checked, len(rows))
}

// names returns the jetstream identifiers an entry resolves to: its
// target's last component and the consumer config fields it sets.
func names(e Entry) []string {
	var out []string
	if e.Target != "" {
		_, member, _ := strings.Cut(e.Target, ".")
		if member == "" {
			member = e.Target
		}
		out = append(out, member)
	}
	for _, f := range e.Fields {
		out = append(out, f.Name)
	}
	return out
}
