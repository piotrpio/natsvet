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

package natsvet_test

import (
	"os"
	"strings"
	"testing"

	"github.com/piotrpio/natsvet"
	"github.com/piotrpio/natsvet/internal/docgen"
)

func TestRulesDocUpToDate(t *testing.T) {
	want := docgen.Generate(natsvet.Analyzers(), natsvet.OptIn())
	got, err := os.ReadFile("docs/rules.md")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Error("docs/rules.md is stale; run `go generate .`")
	}
}

func TestEveryRuleDocumented(t *testing.T) {
	doc := string(docgen.Generate(natsvet.Analyzers(), natsvet.OptIn()))
	for _, a := range natsvet.All() {
		if !containsHeading(doc, a.Name) {
			t.Errorf("rule %s has no section", a.Name)
		}
	}
}

func containsHeading(doc, name string) bool {
	return strings.Contains(doc, "\n## "+name+"\n")
}
