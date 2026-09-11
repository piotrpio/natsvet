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

// Command tablegen writes the natsapi tables derived from the nats.go
// version pinned by the testdata module.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/piotrpio/natsvet/internal/tablegen"
)

func main() {
	testdata := flag.String("testdata", "testdata", "directory of the module that pins nats.go")
	out := flag.String("out", "internal/natsapi", "directory to write the tables to")
	flag.Parse()

	if err := run(*testdata, *out); err != nil {
		fmt.Fprintln(os.Stderr, "tablegen:", err)
		os.Exit(1)
	}
}

func run(testdata, out string) error {
	if err := tablegen.Available(testdata); err != nil {
		return err
	}
	tables, err := tablegen.Collect(testdata)
	if err != nil {
		return err
	}
	gen, err := tables.Generate()
	if err != nil {
		return err
	}
	for name, src := range map[string][]byte{
		tablegen.HeadersFile: gen.Headers,
		tablegen.LegacyFile:  gen.Legacy,
	} {
		if err := os.WriteFile(filepath.Join(out, name), src, 0o644); err != nil {
			return err
		}
	}
	return nil
}
