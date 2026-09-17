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
	"os"

	"github.com/nats-io/nats.go"
)

func main() {
	nc, _ := nats.Connect("")
	if len(os.Args) > 1 {
		nc.Drain() // want `Drain in main followed by process exit drains nothing; Drain returns immediately, wait for the ClosedHandler before exiting`
		os.Exit(0)
	}
	nc.Drain() // want `Drain in main followed by process exit drains nothing; Drain returns immediately, wait for the ClosedHandler before exiting`
	return
}
