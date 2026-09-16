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

// Package docgen renders the rule documentation from the analyzers' Doc
// strings, so what users read is what golangci-lint will show.
package docgen

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// fixes names the rules that offer suggested fixes; analysis.Analyzer has
// no attribute for it.
var fixes = map[string]bool{"headerkey": true}

// Generate renders Markdown for the default-on and opt-in analyzers: a
// table, then one section per rule with the Doc text, its first line as
// the summary and tab-indented lines as a Go code block.
func Generate(defaultOn, optIn []*analysis.Analyzer) []byte {
	var b bytes.Buffer
	b.WriteString("# natsvet rules\n\n")
	b.WriteString("Generated from the analyzers' documentation by `go generate`; do not edit.\n")
	b.WriteString("Default-on rules can be turned off with `-<rule>=false`; opt-in rules are\n")
	b.WriteString("enabled with `-<rule>.enable`.\n\n")
	b.WriteString("| Rule | Default | Fix | Summary |\n|------|---------|-----|---------|\n")
	type entry struct {
		a       *analysis.Analyzer
		enabled string
	}
	var all []entry
	for _, a := range defaultOn {
		all = append(all, entry{a, "on"})
	}
	for _, a := range optIn {
		all = append(all, entry{a, fmt.Sprintf("opt-in (`-%s.enable`)", a.Name)})
	}
	for _, e := range all {
		summary, _ := splitDoc(e.a)
		fmt.Fprintf(&b, "| [`%s`](#%s) | %s | %s | %s |\n", e.a.Name, e.a.Name, e.enabled, yesNo(fixes[e.a.Name]), summary)
	}
	for _, e := range all {
		summary, body := splitDoc(e.a)
		fmt.Fprintf(&b, "\n## %s\n\n", e.a.Name)
		fmt.Fprintf(&b, "%s\n\n", summary)
		fmt.Fprintf(&b, "Default: %s. Fix: %s.\n\n", e.enabled, yesNo(fixes[e.a.Name]))
		b.WriteString(renderBody(body))
	}
	return b.Bytes()
}

// splitDoc returns the summary (the first Doc line without the "name: "
// prefix) and the rest of the Doc.
func splitDoc(a *analysis.Analyzer) (summary, body string) {
	first, rest, _ := strings.Cut(a.Doc, "\n")
	summary = strings.TrimPrefix(first, a.Name+": ")
	return summary, strings.TrimSpace(rest)
}

// renderBody keeps prose paragraphs as they are and wraps runs of
// tab-indented lines (blank lines between them included) in a Go code
// fence.
func renderBody(body string) string {
	lines := strings.Split(body, "\n")
	var b strings.Builder
	for i := 0; i < len(lines); {
		if !strings.HasPrefix(lines[i], "\t") {
			b.WriteString(lines[i] + "\n")
			i++
			continue
		}
		// A code run ends at the last tab-indented line; blank lines
		// inside it are kept, trailing ones are not.
		end := i
		for j := i; j < len(lines); j++ {
			switch {
			case strings.HasPrefix(lines[j], "\t"):
				end = j
			case strings.TrimSpace(lines[j]) != "":
				j = len(lines)
			}
		}
		b.WriteString("```go\n")
		for _, l := range lines[i : end+1] {
			b.WriteString(strings.TrimPrefix(l, "\t") + "\n")
		}
		b.WriteString("```\n")
		i = end + 1
	}
	return b.String()
}

func yesNo(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}
