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

// Package legacyjs reports every use of the legacy JetStream API in package
// nats so a migration to the jetstream package can be inventoried.
package legacyjs

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

const doc = `legacyjs: report uses of the legacy JetStream API (opt-in)

Inventories every use of nats.JetStreamContext, nats.KeyValue,
nats.ObjectStore and their options, configs and message methods, so a
codebase can see what a migration to the jetstream package has to touch.
nats.go does not mark this API deprecated, so no generic deprecation check
sees it. The rule reports facts and never fixes; enable it with
-legacyjs.enable.

	js, _ := nc.JetStream()        // legacy JetStream API: nats.Conn.JetStream
	js.Publish("orders", data)      // legacy JetStream API: nats.JetStream.Publish`

const name = "legacyjs"

var Analyzer = &analysis.Analyzer{
	Name:     name,
	Doc:      doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

var enable bool

func init() {
	Analyzer.Flags.BoolVar(&enable, "enable", false, "report uses of the legacy JetStream API")
}

func run(pass *analysis.Pass) (any, error) {
	if !enable {
		return nil, nil
	}
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	ins.Preorder([]ast.Node{(*ast.Ident)(nil)}, func(n ast.Node) {
		id := n.(*ast.Ident)
		sym, ok := natsapi.LegacySymbol(pass.TypesInfo.Uses[id])
		if !ok {
			return
		}
		pass.Report(analysis.Diagnostic{
			Pos:      id.Pos(),
			End:      id.End(),
			Category: name,
			Message:  "legacy JetStream API: " + sym + "; see the jetstream package",
		})
	})
	return nil, nil
}
