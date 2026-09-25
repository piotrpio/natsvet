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
// version pinned by the testdata module, and the nats-server header list
// derived from a nats-server checkout.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/piotrpio/natsvet/internal/tablegen"
)

func main() {
	testdata := flag.String("testdata", "testdata", "directory of the module that pins nats.go")
	server := flag.String("server", "", "nats-server checkout for the header list (default $NATS_SERVER_DIR, then ~/.cache/natsvet-corpus/nats-server)")
	only := flag.String("only", "", `write only this table: "server"`)
	out := flag.String("out", "internal/natsapi", "directory to write the tables to")
	flag.Parse()

	if err := run(*testdata, *server, *only, *out); err != nil {
		fmt.Fprintln(os.Stderr, "tablegen:", err)
		os.Exit(1)
	}
}

func run(testdata, server, only, out string) error {
	switch only {
	case "":
		if err := writeNATSTables(testdata, out); err != nil {
			return err
		}
		return writeServerHeaders(server, out, false)
	case "server":
		return writeServerHeaders(server, out, true)
	}
	return fmt.Errorf("-only %q: the only selectable table is \"server\"", only)
}

func writeNATSTables(testdata, out string) error {
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
		tablegen.HeadersFile:   gen.Headers,
		tablegen.LegacyFile:    gen.Legacy,
		tablegen.DurationsFile: gen.Durations,
	} {
		if err := os.WriteFile(filepath.Join(out, name), src, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// writeServerHeaders regenerates the nats-server header list. Without a
// checkout it keeps the committed table unless required is set.
func writeServerHeaders(server, out string, required bool) error {
	if server == "" {
		server = os.Getenv("NATS_SERVER_DIR")
	}
	if server == "" {
		if home, err := os.UserHomeDir(); err == nil {
			server = filepath.Join(home, ".cache", "natsvet-corpus", "nats-server")
		}
	}
	if _, err := os.Stat(filepath.Join(server, "server")); errors.Is(err, fs.ErrNotExist) && !required {
		fmt.Fprintf(os.Stderr, "tablegen: no nats-server checkout at %s; %s left unchanged (set NATS_SERVER_DIR)\n", server, tablegen.ServerHeadersFile)
		return nil
	}
	names, version, err := tablegen.ServerHeaders(server)
	if err != nil {
		return err
	}
	src, err := tablegen.RenderServerHeaders(names, version)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, tablegen.ServerHeadersFile), src, 0o644)
}
