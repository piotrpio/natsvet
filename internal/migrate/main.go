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

// Package migrate plans the move of a module off the legacy JetStream API
// (nats.JetStreamContext, nats.KeyValue, nats.ObjectStore) onto the
// jetstream package. It never edits code.
package migrate

import (
	_ "embed"
	"flag"
	"fmt"
	"io"
)

//go:embed SKILL.md
var skill string

const usage = `usage:
  natsvet migrate plan [-decisions file] [-format json|markdown] [-tests=bool] packages...
  natsvet migrate skill
`

// Main runs `natsvet migrate` with args (after "migrate") and returns the
// exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "skill":
		if len(args) != 1 {
			fmt.Fprint(stderr, usage)
			return 2
		}
		fmt.Fprint(stdout, skill)
		return 0
	case "plan":
	default:
		fmt.Fprint(stderr, usage)
		return 2
	}
	fs := flag.NewFlagSet("natsvet migrate plan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	decisions := fs.String("decisions", "", "answers file (default: "+decisionsFile+" at the module root, when it exists)")
	format := fs.String("format", "json", "output format: json or markdown")
	tests := fs.Bool("tests", true, "include test files")
	fs.Usage = func() { fmt.Fprint(stderr, usage) }
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if *format != "json" && *format != "markdown" {
		fmt.Fprintf(stderr, "natsvet migrate plan: unknown format %q\n", *format)
		return 2
	}
	patterns := fs.Args()
	if len(patterns) == 0 {
		patterns = []string{"."}
	}
	plan, err := buildPlan(options{dir: ".", patterns: patterns, tests: *tests, decisions: *decisions})
	if err != nil {
		fmt.Fprintf(stderr, "natsvet migrate plan: %v\n", err)
		return 1
	}
	if *format == "markdown" {
		err = writeMarkdown(stdout, plan)
	} else {
		err = writeJSON(stdout, plan)
	}
	if err != nil {
		fmt.Fprintf(stderr, "natsvet migrate plan: %v\n", err)
		return 1
	}
	return 0
}
