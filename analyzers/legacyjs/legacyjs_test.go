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

package legacyjs

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	if err := Analyzer.Flags.Set("enable", "true"); err != nil {
		t.Fatal(err)
	}
	analysistest.Run(t, "../../testdata", Analyzer, "natsvet/testdata/legacyjs")
}

func TestDisabledByDefault(t *testing.T) {
	if err := Analyzer.Flags.Set("enable", "false"); err != nil {
		t.Fatal(err)
	}
	// With the rule disabled every // want comment in the package is an
	// unexpected expectation, so a passing run here would mean the rule
	// reported anyway; the diagnostics slice is what we check.
	results := analysistest.Run(&quiet{t}, "../../testdata", Analyzer, "natsvet/testdata/legacyjs")
	for _, r := range results {
		if len(r.Diagnostics) != 0 {
			t.Errorf("disabled rule reported %d diagnostics", len(r.Diagnostics))
		}
	}
}

// quiet swallows the expectation failures analysistest raises for // want
// comments that the disabled rule does not satisfy.
type quiet struct{ *testing.T }

func (quiet) Errorf(string, ...any) {}
