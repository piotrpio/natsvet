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

// Command docgen writes docs/rules.md from the registered analyzers.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/piotrpio/natsvet"
	"github.com/piotrpio/natsvet/internal/docgen"
)

func main() {
	out := flag.String("out", "docs/rules.md", "file to write")
	flag.Parse()
	if err := os.WriteFile(*out, docgen.Generate(natsvet.Analyzers(), natsvet.OptIn()), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "docgen:", err)
		os.Exit(1)
	}
}
