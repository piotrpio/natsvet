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

// Package ctxdeadline reports context.Background() or context.TODO()
// passed directly to nats.go calls that need a deadline.
package ctxdeadline

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

const doc = `ctxdeadline: report context.Background() or context.TODO() passed where a deadline is needed

FlushWithContext returns ErrNoDeadlineContext for such a context every
time; RequestWithContext, RequestMsgWithContext and NextMsgWithContext
accept it and then block forever whenever a responder exists but never
replies; legacy Fetch and FetchBatch return ErrNoDeadlineContext for
nats.Context(context.Background()). Only a direct Background()/TODO()
argument is reported.

	nc.RequestWithContext(context.Background(), "s", data)    // may block forever
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)`

const name = "ctxdeadline"

var Analyzer = &analysis.Analyzer{
	Name:     name,
	Doc:      doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

const contextPkg natsapi.Pkg = "context"

type hook struct {
	recv string
	name string
	msg  string
}

var hooks = []hook{
	{"Conn", "FlushWithContext", "FlushWithContext requires a context with a deadline (returns ErrNoDeadlineContext); use context.WithTimeout"},
	{"Conn", "RequestWithContext", "RequestWithContext with a context that has no deadline blocks forever if a responder exists but never replies; use context.WithTimeout"},
	{"Conn", "RequestMsgWithContext", "RequestMsgWithContext with a context that has no deadline blocks forever if a responder exists but never replies; use context.WithTimeout"},
	{"Subscription", "NextMsgWithContext", "NextMsgWithContext with a context that has no deadline blocks forever if a responder exists but never replies; use context.WithTimeout"},
}

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	info := pass.TypesInfo
	// background returns the name of the context constructor when e is a
	// direct call to context.Background or context.TODO.
	background := func(e ast.Expr) (string, bool) {
		call, ok := ast.Unparen(e).(*ast.CallExpr)
		if !ok {
			return "", false
		}
		fn := natsapi.Callee(info, call)
		for _, n := range []string{"Background", "TODO"} {
			if natsapi.IsFunc(fn, contextPkg, n) {
				return n, true
			}
		}
		return "", false
	}
	ins.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
		call := n.(*ast.CallExpr)
		fn := natsapi.Callee(info, call)
		if fn == nil || len(call.Args) == 0 {
			return
		}
		for _, h := range hooks {
			if natsapi.IsMethod(fn, natsapi.Core, h.recv, h.name) {
				if _, ok := background(call.Args[0]); ok {
					pass.Report(analysis.Diagnostic{Pos: call.Args[0].Pos(), End: call.Args[0].End(), Category: name, Message: h.msg})
				}
				return
			}
		}
		if !natsapi.IsMethod(fn, natsapi.Core, "Subscription", "Fetch") && !natsapi.IsMethod(fn, natsapi.Core, "Subscription", "FetchBatch") {
			return
		}
		for _, a := range call.Args {
			opt, ok := ast.Unparen(a).(*ast.CallExpr)
			if !ok || !natsapi.IsFunc(natsapi.Callee(info, opt), natsapi.Core, "Context") || len(opt.Args) != 1 {
				continue
			}
			if ctor, ok := background(opt.Args[0]); ok {
				pass.Report(analysis.Diagnostic{
					Pos: a.Pos(), End: a.End(), Category: name,
					Message: fmt.Sprintf("%s with nats.Context(context.%s()) returns ErrNoDeadlineContext; use nats.MaxWait or a context with a deadline", fn.Name(), ctor),
				})
			}
		}
	})
	return nil, nil
}
