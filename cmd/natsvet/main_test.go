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

package main

import (
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestMain runs natsvet itself when the test binary is re-executed by
// run.
func TestMain(m *testing.M) {
	if os.Getenv("NATSVET_TEST_MAIN") == "1" {
		os.Args = append([]string{"natsvet"}, os.Args[1:]...)
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// run executes natsvet with args and returns its combined output and exit
// code.
func run(t *testing.T, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(), "NATSVET_TEST_MAIN=1")
	out, err := cmd.CombinedOutput()
	code := 0
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code = exit.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return string(out), code
}

// Agents learn what a tool does from its help: every help entry point
// names the migration planner.
func TestHelpNamesMigrate(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
		also []string
	}{
		{[]string{"-h"}, 0, []string{"-legacyjs.enable", "natsvet help migrate", "natsvet migrate apply"}},
		{[]string{"--help"}, 0, []string{"-legacyjs.enable"}},
		{[]string{"help"}, 0, []string{"Registered analyzers"}},
		{nil, 1, []string{"natsvet help"}},
		{[]string{"help", "migrate"}, 0, []string{"-decisions", "natsvet-migrate.json", "migrate skill", "migrate apply", "-component", "-dry-run"}},
		{[]string{"migrate", "apply", "-h"}, 0, []string{"-dry-run"}},
		{[]string{"migrate", "--help"}, 0, []string{"-format"}},
		{[]string{"migrate", "plan", "-h"}, 0, []string{"-tests"}},
		{[]string{"migrate"}, 2, []string{"migrate plan"}},
		{[]string{"migrate", "bogus"}, 2, []string{`unknown command "bogus"`}},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			out, code := run(t, tc.args...)
			if code != tc.code {
				t.Errorf("exit %d, want %d:\n%s", code, tc.code, out)
			}
			for _, want := range append([]string{"natsvet migrate plan"}, tc.also...) {
				if !strings.Contains(out, want) {
					t.Errorf("output lacks %q:\n%s", want, out)
				}
			}
		})
	}
}
