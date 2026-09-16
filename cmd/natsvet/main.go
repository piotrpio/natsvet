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
// version and revision.
package main

import (
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	"github.com/piotrpio/natsvet"
	"golang.org/x/tools/go/analysis/multichecker"
)

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "-version" || os.Args[1] == "--version") {
		fmt.Println(version())
		return
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
