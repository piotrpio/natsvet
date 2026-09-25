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
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

//go:embed SKILL.md
var skill string

// usage describes the migrate command; %[1]s is the program name.
const usage = `%[1]s migrate plans the move of a module off the legacy JetStream API
(nats.JetStreamContext, nats.KeyValue, nats.ObjectStore) onto the jetstream
package. It reads the code and never edits it.

Usage:

    %[1]s migrate plan [flags] [packages]   write the plan (packages default to .)
    %[1]s migrate skill                     print the agent skill that follows a plan

The plan lists every legacy site as mechanical (exact replacement and byte-offset
edits), guided (a template and the facts it needs), a decision for the user (with
a behavior-preserving default) or unmapped, and orders the work into steps after
each of which the module compiles. Answers to its decisions are read from
natsvet-migrate.json at the module root. Agents: read '%[1]s migrate skill'
before acting on a plan.

Flags of plan:

`

// planFlags declares the flags of migrate plan.
type planFlags struct {
	decisions, format string
	tests             bool
}

func newPlanFlags(stderr io.Writer) (*flag.FlagSet, *planFlags) {
	fs := flag.NewFlagSet("natsvet migrate plan", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pf := &planFlags{}
	fs.StringVar(&pf.decisions, "decisions", "", "answers file (default: "+decisionsFile+" at the module root, when it exists)")
	fs.StringVar(&pf.format, "format", "json", "output format: json or markdown")
	fs.BoolVar(&pf.tests, "tests", true, "include test files")
	return fs, pf
}

// printUsage writes the migrate usage with the flags of plan.
func printUsage(w io.Writer) {
	fmt.Fprintf(w, usage, progname())
	fs, _ := newPlanFlags(w)
	fs.SetOutput(w)
	fs.PrintDefaults()
}

func progname() string { return filepath.Base(os.Args[0]) }

// Main runs `natsvet migrate` with args (after "migrate") and returns the
// exit code.
func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}
	switch args[0] {
	case "help", "-h", "-help", "--help":
		printUsage(stdout)
		return 0
	case "skill":
		if len(args) != 1 {
			printUsage(stderr)
			return 2
		}
		fmt.Fprint(stdout, skill)
		return 0
	case "plan":
	default:
		fmt.Fprintf(stderr, "%s migrate: unknown command %q\n\n", progname(), args[0])
		printUsage(stderr)
		return 2
	}
	fs, pf := newPlanFlags(stderr)
	fs.Usage = func() { printUsage(stderr) }
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if pf.format != "json" && pf.format != "markdown" {
		fmt.Fprintf(stderr, "%s migrate plan: unknown format %q\n", progname(), pf.format)
		return 2
	}
	patterns := fs.Args()
	if len(patterns) == 0 {
		patterns = []string{"."}
	}
	plan, err := buildPlan(options{dir: ".", patterns: patterns, tests: pf.tests, decisions: pf.decisions})
	if err != nil {
		fmt.Fprintf(stderr, "%s migrate plan: %v\n", progname(), err)
		return 1
	}
	if pf.format == "markdown" {
		err = writeMarkdown(stdout, plan)
	} else {
		err = writeJSON(stdout, plan)
	}
	if err != nil {
		fmt.Fprintf(stderr, "%s migrate plan: %v\n", progname(), err)
		return 1
	}
	return 0
}
