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

// Package drain reports Close called right after Drain on the same
// connection.
package drain

import (
	"go/ast"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

const doc = `drain: report Close called right after Drain

Drain returns as soon as it has started draining in a goroutine; the
ClosedHandler reports when it finishes. A Close in the next statement, or a
deferred Close that runs when the function returns from a trailing Drain,
closes the connection and discards the drain in progress. main and test
functions are exempt from the deferred form, where process exit dominates.

	nc.Drain()
	nc.Close()          // aborts the drain

	defer nc.Close()
	return nc.Drain()   // same`

const name = "drain"

var Analyzer = &analysis.Analyzer{
	Name:     name,
	Doc:      doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

const (
	adjacentMsg = "Close immediately after Drain aborts the drain; wait for the ClosedHandler instead"
	deferredMsg = "deferred Close runs as soon as Drain returns and aborts the drain; wait for the ClosedHandler before returning"
)

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	info := pass.TypesInfo
	// connCall returns the receiver object when e is <id>.<method>() on a
	// *nats.Conn for the given method.
	connCall := func(e ast.Expr, method string) (types.Object, *ast.CallExpr) {
		call, ok := ast.Unparen(e).(*ast.CallExpr)
		if !ok || !natsapi.IsMethod(natsapi.Callee(info, call), natsapi.Core, "Conn", method) {
			return nil, nil
		}
		sel := call.Fun.(*ast.SelectorExpr)
		id, ok := ast.Unparen(sel.X).(*ast.Ident)
		if !ok {
			return nil, nil
		}
		return info.ObjectOf(id), call
	}
	report := func(call *ast.CallExpr, msg string) {
		pass.Report(analysis.Diagnostic{Pos: call.Pos(), End: call.End(), Category: name, Message: msg})
	}

	ins.Preorder([]ast.Node{(*ast.BlockStmt)(nil), (*ast.CaseClause)(nil), (*ast.CommClause)(nil)}, func(n ast.Node) {
		checkAdjacent(info, blockList(n), connCall, report)
	})
	ins.Preorder([]ast.Node{(*ast.FuncDecl)(nil), (*ast.FuncLit)(nil)}, func(n ast.Node) {
		var body *ast.BlockStmt
		switch n := n.(type) {
		case *ast.FuncDecl:
			if n.Body == nil || exempt(pass, n) {
				return
			}
			body = n.Body
		case *ast.FuncLit:
			body = n.Body
		}
		checkDeferred(body, connCall, report)
	})
	return nil, nil
}

func blockList(n ast.Node) []ast.Stmt {
	switch n := n.(type) {
	case *ast.BlockStmt:
		return n.List
	case *ast.CaseClause:
		return n.Body
	case *ast.CommClause:
		return n.Body
	}
	return nil
}

type connCallFunc func(ast.Expr, string) (types.Object, *ast.CallExpr)

// drainStmt returns the connection object when s is x.Drain() as an
// expression statement, the right-hand side of an assignment, or the init
// of an if statement (in which case the if body is the error check and
// tolerated is false).
func drainStmt(s ast.Stmt, connCall connCallFunc) (obj types.Object, tolerateIf bool) {
	switch s := s.(type) {
	case *ast.ExprStmt:
		obj, _ = connCall(s.X, "Drain")
		return obj, true
	case *ast.AssignStmt:
		if len(s.Rhs) == 1 {
			obj, _ = connCall(s.Rhs[0], "Drain")
		}
		return obj, true
	case *ast.IfStmt:
		if as, ok := s.Init.(*ast.AssignStmt); ok && len(as.Rhs) == 1 {
			obj, _ = connCall(as.Rhs[0], "Drain")
		}
		return obj, false
	}
	return nil, false
}

func checkAdjacent(info *types.Info, list []ast.Stmt, connCall connCallFunc, report func(*ast.CallExpr, string)) {
	for i, s := range list {
		obj, tolerateIf := drainStmt(s, connCall)
		if obj == nil {
			continue
		}
		j := i + 1
		if tolerateIf && j < len(list) {
			if ifs, ok := list[j].(*ast.IfStmt); ok && !callsMethodOn(info, ifs, obj) {
				j++
			}
		}
		if j >= len(list) {
			continue
		}
		es, ok := list[j].(*ast.ExprStmt)
		if !ok {
			continue
		}
		if closeObj, call := connCall(es.X, "Close"); closeObj == obj {
			report(call, adjacentMsg)
		}
	}
}

// callsMethodOn reports whether n contains a method call whose receiver is
// the variable obj.
func callsMethodOn(info *types.Info, n ast.Node, obj types.Object) bool {
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || found {
			return !found
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
			if id, ok := ast.Unparen(sel.X).(*ast.Ident); ok && info.ObjectOf(id) == obj {
				found = true
			}
		}
		return true
	})
	return found
}

func checkDeferred(body *ast.BlockStmt, connCall connCallFunc, report func(*ast.CallExpr, string)) {
	deferred := make(map[types.Object]bool)
	for _, s := range body.List {
		if d, ok := s.(*ast.DeferStmt); ok {
			if obj, _ := connCall(d.Call, "Close"); obj != nil {
				deferred[obj] = true
			}
		}
	}
	if len(deferred) == 0 {
		return
	}
	last := len(body.List) - 1
	for i, s := range body.List {
		var obj types.Object
		var call *ast.CallExpr
		switch s := s.(type) {
		case *ast.ExprStmt:
			if i != last {
				if _, isReturn := body.List[i+1].(*ast.ReturnStmt); !isReturn || i+1 != last {
					continue
				}
			}
			obj, call = connCall(s.X, "Drain")
		case *ast.ReturnStmt:
			if i != last || len(s.Results) != 1 {
				continue
			}
			obj, call = connCall(s.Results[0], "Drain")
		}
		if obj != nil && deferred[obj] {
			report(call, deferredMsg)
		}
	}
}

// exempt reports whether fd is main in package main or a test, benchmark
// or fuzz function, where process exit makes the deferred Close moot.
func exempt(pass *analysis.Pass, fd *ast.FuncDecl) bool {
	if fd.Recv != nil {
		return false
	}
	n := fd.Name.Name
	if n == "main" && pass.Pkg.Name() == "main" {
		return true
	}
	file := pass.Fset.File(fd.Pos())
	if file == nil || !strings.HasSuffix(file.Name(), "_test.go") {
		return false
	}
	for _, prefix := range []string{"Test", "Benchmark", "Fuzz"} {
		if strings.HasPrefix(n, prefix) {
			return true
		}
	}
	return false
}
