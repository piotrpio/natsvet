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

// Package pubasync reports a discarded PubAckFuture in a package with no
// other way to learn that an async publish failed.
package pubasync

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

const doc = `pubasync: report a discarded PubAckFuture with no async error path in the package

The future's Err channel and the handler installed by
WithPublishAsyncErrHandler are the only two places a rejected or timed-out
async publish is ever reported; when the future is thrown away and the
package neither installs the handler nor waits on PublishAsyncComplete, the
publish fails silently and the caller believes the message was stored. An
ack handler (WithPublishAsyncAckHandler) runs only for successful publishes
and does not replace the error handler.

	_, err := js.PublishAsync("orders.new", data)   // a NACK is never seen
	f, err := js.PublishAsync("orders.new", data)
	select {
	case <-f.Ok():
	case err := <-f.Err():
	}`

const name = "pubasync"

var Analyzer = &analysis.Analyzer{
	Name:     name,
	Doc:      doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	info := pass.TypesInfo
	if natsapi.PackageUses(info, natsapi.JetStream, "", "WithPublishAsyncErrHandler") ||
		natsapi.PackageUses(info, natsapi.Core, "", "PublishAsyncErrHandler") ||
		natsapi.PackageUses(info, natsapi.JetStream, "Publisher", "PublishAsyncComplete") ||
		natsapi.PackageUses(info, natsapi.Core, "JetStream", "PublishAsyncComplete") {
		return nil, nil
	}
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	ins.WithStack([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return false
		}
		call := n.(*ast.CallExpr)
		fn := natsapi.Callee(info, call)
		for _, method := range []string{"PublishAsync", "PublishMsgAsync"} {
			if !natsapi.IsMethod(fn, natsapi.JetStream, "Publisher", method) && !natsapi.IsMethod(fn, natsapi.Core, "JetStream", method) {
				continue
			}
			if natsapi.Discarded(stack) {
				pass.Report(analysis.Diagnostic{
					Pos: call.Pos(), End: call.End(), Category: name,
					Message: fmt.Sprintf("PubAckFuture from %s discarded and this package never sets an async error handler or awaits PublishAsyncComplete; publish errors are lost", method),
				})
			}
			return true
		}
		return true
	})
	return nil, nil
}
