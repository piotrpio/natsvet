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

// Package msgloop reports loops over JetStream messages that mishandle the
// iterator's terminal state.
package msgloop

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

const doc = `msgloop: report a Next loop that never exits and a Fetch batch ranged without Error

A for loop that calls MessagesContext.Next and continues on every error never
exits once the iterator is stopped or drained, because Next then returns
ErrMsgIteratorClosed on every call without blocking. A Fetch result whose
Messages channel is ranged over without checking Error afterwards drops the
batch's terminal error, so a missed heartbeat or a deleted consumer looks
like an empty batch.

	for {
		msg, err := it.Next()
		if err != nil {
			log.Println(err)
			continue           // forever, once it.Stop() has run
		}
		msg.Ack()
	}

	msgs, _ := cons.Fetch(10)
	for msg := range msgs.Messages() {
		msg.Ack()
	}
	if err := msgs.Error(); err != nil { // the part that is missing
		return err
	}`

const name = "msgloop"

var Analyzer = &analysis.Analyzer{
	Name:     name,
	Doc:      doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

const neverExits = "loop continues on every Next error; after Stop or Drain, Next returns ErrMsgIteratorClosed on every call and the loop never exits"

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	ins.WithStack([]ast.Node{(*ast.ForStmt)(nil), (*ast.RangeStmt)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return false
		}
		switch n := n.(type) {
		case *ast.ForStmt:
			if n.Init == nil && n.Cond == nil && n.Post == nil {
				checkLoop(pass, n, loopLabel(stack))
			}
		case *ast.RangeStmt:
			checkRange(pass, n, natsapi.EnclosingFuncBody(stack))
		}
		return true
	})
	return nil, nil
}

// loopLabel returns the label of the loop at the top of stack, or "".
func loopLabel(stack []ast.Node) string {
	if len(stack) >= 2 {
		if l, ok := stack[len(stack)-2].(*ast.LabeledStmt); ok {
			return l.Label.Name
		}
	}
	return ""
}

// checkLoop reports an if err != nil right after a MessagesContext.Next
// assignment whose then-branch never leaves the loop.
func checkLoop(pass *analysis.Pass, loop *ast.ForStmt, label string) {
	info := pass.TypesInfo
	if mentionsClosed(info, loop.Body) {
		return
	}
	list := loop.Body.List
	for i, s := range list {
		var ifs *ast.IfStmt
		var errObj types.Object
		switch s := s.(type) {
		case *ast.AssignStmt:
			if i+1 >= len(list) {
				continue
			}
			next, ok := list[i+1].(*ast.IfStmt)
			if !ok {
				continue
			}
			ifs, errObj = next, nextError(info, s)
		case *ast.IfStmt:
			init, ok := s.Init.(*ast.AssignStmt)
			if !ok {
				continue
			}
			ifs, errObj = s, nextError(info, init)
		}
		if errObj == nil || !isNilCheck(info, ifs.Cond, errObj) || leavesLoop(info, ifs.Body, label) {
			continue
		}
		if !endsInContinue(ifs.Body, label) && ifs.Else == nil {
			continue
		}
		pass.Report(analysis.Diagnostic{Pos: ifs.Pos(), End: ifs.Cond.End(), Category: name, Message: neverExits})
	}
}

// nextError returns the error variable of msg, err := it.Next(...) when the
// callee is MessagesContext.Next, or nil.
func nextError(info *types.Info, a *ast.AssignStmt) types.Object {
	if len(a.Lhs) != 2 || len(a.Rhs) != 1 {
		return nil
	}
	call, ok := ast.Unparen(a.Rhs[0]).(*ast.CallExpr)
	if !ok || !natsapi.IsMethod(natsapi.Callee(info, call), natsapi.JetStream, "MessagesContext", "Next") {
		return nil
	}
	id, ok := a.Lhs[1].(*ast.Ident)
	if !ok || id.Name == "_" {
		return nil
	}
	return info.ObjectOf(id)
}

// isNilCheck reports whether cond is errObj != nil in either order.
func isNilCheck(info *types.Info, cond ast.Expr, errObj types.Object) bool {
	b, ok := ast.Unparen(cond).(*ast.BinaryExpr)
	if !ok || b.Op != token.NEQ {
		return false
	}
	isErr := func(e ast.Expr) bool {
		id, ok := ast.Unparen(e).(*ast.Ident)
		return ok && info.Uses[id] == errObj
	}
	isNil := func(e ast.Expr) bool {
		id, ok := ast.Unparen(e).(*ast.Ident)
		if !ok {
			return false
		}
		_, ok = info.Uses[id].(*types.Nil)
		return ok
	}
	return isErr(b.X) && isNil(b.Y) || isNil(b.X) && isErr(b.Y)
}

// leavesLoop reports whether body contains a statement that leaves the
// enclosing loop: a return, break, goto, a continue of an outer loop, or a
// call that exits the process.
func leavesLoop(info *types.Info, body *ast.BlockStmt, label string) bool {
	leaves := false
	ast.Inspect(body, func(n ast.Node) bool {
		if leaves {
			return false
		}
		switch n := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			leaves = true
		case *ast.BranchStmt:
			switch n.Tok {
			case token.BREAK, token.GOTO:
				leaves = true
			case token.CONTINUE:
				leaves = n.Label != nil && n.Label.Name != label
			}
		case *ast.CallExpr:
			leaves = natsapi.ExitsProcess(info, n)
		}
		return !leaves
	})
	return leaves
}

// endsInContinue reports whether the last statement of body is a continue
// of the enclosing loop.
func endsInContinue(body *ast.BlockStmt, label string) bool {
	if len(body.List) == 0 {
		return false
	}
	b, ok := body.List[len(body.List)-1].(*ast.BranchStmt)
	return ok && b.Tok == token.CONTINUE && (b.Label == nil || b.Label.Name == label)
}

// mentionsClosed reports whether any identifier in n refers to
// jetstream.ErrMsgIteratorClosed.
func mentionsClosed(info *types.Info, n ast.Node) bool {
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if ok && id.Name == "ErrMsgIteratorClosed" && natsapi.IsPkg(info.Uses[id], natsapi.JetStream) {
			found = true
		}
		return !found
	})
	return found
}

var fetches = []struct {
	pkg    natsapi.Pkg
	recv   string
	method string
}{
	{natsapi.JetStream, "Consumer", "Fetch"},
	{natsapi.JetStream, "Consumer", "FetchBytes"},
	{natsapi.JetStream, "Consumer", "FetchNoWait"},
	{natsapi.Core, "Subscription", "FetchBatch"},
}

// checkRange reports for msg := range r.Messages() when r is a local batch
// from a fetch call whose Error is never read in the function.
func checkRange(pass *analysis.Pass, rs *ast.RangeStmt, body *ast.BlockStmt) {
	info := pass.TypesInfo
	call, ok := ast.Unparen(rs.X).(*ast.CallExpr)
	if !ok || body == nil {
		return
	}
	fn := natsapi.Callee(info, call)
	if !natsapi.IsMethod(fn, natsapi.JetStream, "MessageBatch", "Messages") && !natsapi.IsMethod(fn, natsapi.Core, "MessageBatch", "Messages") {
		return
	}
	id, ok := ast.Unparen(call.Fun.(*ast.SelectorExpr).X).(*ast.Ident)
	if !ok {
		return
	}
	def, ok := natsapi.SingleDefinition(info, body, id)
	if !ok {
		return
	}
	defCall, ok := ast.Unparen(def).(*ast.CallExpr)
	if !ok {
		return
	}
	defFn := natsapi.Callee(info, defCall)
	method := ""
	for _, f := range fetches {
		if natsapi.IsMethod(defFn, f.pkg, f.recv, f.method) {
			method = f.method
		}
	}
	if method == "" {
		return
	}
	obj := info.Uses[id]
	if obj == nil {
		return
	}
	// Every use of the batch must be r.Messages() or r.Error(); anything
	// else hands it to code that may read Error.
	allowed := map[*ast.Ident]bool{}
	readsError := false
	ast.Inspect(body, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		x, ok := ast.Unparen(sel.X).(*ast.Ident)
		if !ok || info.Uses[x] != obj {
			return true
		}
		switch sel.Sel.Name {
		case "Error":
			readsError = true
			allowed[x] = true
		case "Messages":
			allowed[x] = true
		}
		return true
	})
	if readsError {
		return
	}
	escaped := false
	ast.Inspect(body, func(n ast.Node) bool {
		if x, ok := n.(*ast.Ident); ok && info.Uses[x] == obj && !allowed[x] {
			escaped = true
		}
		return !escaped
	})
	if escaped {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos: rs.Pos(), End: rs.X.End(), Category: name,
		Message: fmt.Sprintf("%s result ranged without checking Error(); a failed fetch looks like an empty batch", method),
	})
}
