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

// Command natsvet runs the natsvet analyzers. It works standalone
// (natsvet ./..., natsvet -fix ./...), as go vet -vettool=natsvet, and as
// go fix -fixtool=natsvet. natsvet -version prints the build's module
// version and revision. natsvet migrate plans the move off the legacy
// JetStream API (natsvet migrate plan ./...), applies the plan's machine
// steps one at a time (natsvet migrate apply ./...) and prints the agent
// skill that follows the plan (natsvet migrate skill).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/piotrpio/natsvet"
	"github.com/piotrpio/natsvet/internal/migrate"
	"golang.org/x/tools/go/analysis/multichecker"
)

// usage lists every way to run natsvet; the analyzer framework prints
// only its own flags and analyzers.
const usage = `%[1]s reports misuse of the nats.go client API, and plans the move off the
legacy JetStream API (nats.JetStreamContext, nats.KeyValue, nats.ObjectStore)
onto the jetstream package.

Usage:

    %[1]s [flags] packages                    run the analyzers (-fix applies fixes)
    go vet -vettool=$(which %[1]s) packages   run the analyzers through go vet
    go fix -fixtool=$(which %[1]s) packages   apply the analyzers' fixes through go fix
    %[1]s migrate plan [flags] [packages]     plan the migration off the legacy JetStream API
    %[1]s migrate apply [flags] [packages]    apply the plan's next machine step
    %[1]s migrate skill                       print the agent skill that follows a plan
    %[1]s help [analyzer | migrate]           describe the analyzers, one analyzer, or migrate
    %[1]s -version                            print the version
`

func main() {
	progname := filepath.Base(os.Args[0])
	args := os.Args[1:]
	switch {
	case len(args) == 1 && (args[0] == "-version" || args[0] == "--version"):
		fmt.Println(version())
		return
	case len(args) >= 1 && args[0] == "migrate":
		// go vet and go fix pass flags and package config files, never
		// "migrate", first.
		os.Exit(migrate.Main(args[1:], os.Stdout, os.Stderr))
	case len(args) == 2 && args[0] == "help" && args[1] == "migrate":
		os.Exit(migrate.Main([]string{"help"}, os.Stdout, os.Stderr))
	case len(args) == 0:
		fmt.Fprintf(os.Stderr, usage+"\nRun '%[1]s help' for the analyzers and their flags.\n", progname)
		os.Exit(1)
	case len(args) == 1 && args[0] == "help":
		// The analyzer framework's help follows.
		fmt.Printf(usage+"\n", progname)
	}
	flag.Usage = func() {
		out := flag.CommandLine.Output()
		fmt.Fprintf(out, usage+"\nFlags:\n\n", progname)
		flag.PrintDefaults()
		fmt.Fprintf(out, "\nRun '%[1]s help' for the analyzers, '%[1]s help migrate' for the migration planner.\n", progname)
	}
	multichecker.Main(natsvet.All()...)
}

// version describes the build from the information the Go toolchain embeds:
// the module version (a tag, a pseudo-version, or "(devel)" for a checkout
// build) and, when known, the VCS revision and whether the tree was
// modified.
func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "natsvet (unknown build)"
	}
	var b strings.Builder
	b.WriteString("natsvet " + info.Main.Version)
	var rev, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if rev != "" {
		if len(rev) > 12 {
			rev = rev[:12]
		}
		b.WriteString(" (" + rev)
		if modified == "true" {
			b.WriteString(", modified")
		}
		b.WriteString(")")
	}
	return b.String()
}
