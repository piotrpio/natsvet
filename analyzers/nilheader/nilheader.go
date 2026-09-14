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

// Package nilheader reports header writes on a nats.Msg built as a
// composite literal without a Header.
package nilheader

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

const doc = `nilheader: report header writes on a nats.Msg literal that has no Header

nats.Header.Set, Add and a direct Header[key] = assignment are plain map
writes; nats.NewMsg allocates the map and a composite literal does not, so
the write panics with an assignment to a nil map. The rule follows the
message variable back to its single definition in the same function.

	m := &nats.Msg{Subject: "s"}
	m.Header.Set("X-Id", "1")   // panic: assignment to entry in nil map
	m := nats.NewMsg("s")       // allocates Header`

const name = "nilheader"

var Analyzer = &analysis.Analyzer{
	Name:     name,
	Doc:      doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

var msgType = natsapi.TypeRef{Pkg: natsapi.Core, Name: "Msg"}

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	info := pass.TypesInfo
	// headerOf returns the message identifier when e is <id>.Header on a
	// nats.Msg.
	headerOf := func(e ast.Expr) (*ast.Ident, bool) {
		sel, ok := ast.Unparen(e).(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Header" {
			return nil, false
		}
		f, ok := info.Uses[sel.Sel].(*types.Var)
		if !ok || !f.IsField() || !natsapi.IsPkg(f, natsapi.Core) {
			return nil, false
		}
		id, ok := ast.Unparen(sel.X).(*ast.Ident)
		return id, ok
	}
	// headerless reports whether id's single definition is a nats.Msg
	// literal without a Header key and the function never assigns id.Header.
	headerless := func(body *ast.BlockStmt, id *ast.Ident) bool {
		def, ok := natsapi.SingleDefinition(info, body, id)
		if !ok {
			return false
		}
		if u, isAddr := ast.Unparen(def).(*ast.UnaryExpr); isAddr {
			def = u.X
		}
		lit, ok := ast.Unparen(def).(*ast.CompositeLit)
		if !ok {
			return false
		}
		fields, _, ok := natsapi.CompositeFields(info, lit, msgType)
		if !ok {
			return false
		}
		if _, has := fields["Header"]; has {
			return false
		}
		return !assignsHeader(body, info, info.ObjectOf(id))
	}
	report := func(n ast.Node, what string) {
		pass.Report(analysis.Diagnostic{
			Pos: n.Pos(), End: n.End(), Category: name,
			Message: fmt.Sprintf("%s on a nats.Msg literal without Header panics (nil map); use nats.NewMsg or set Header: nats.Header{}", what),
		})
	}
	ins.WithStack([]ast.Node{(*ast.CallExpr)(nil), (*ast.AssignStmt)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return false
		}
		body := natsapi.EnclosingFuncBody(stack)
		switch n := n.(type) {
		case *ast.CallExpr:
			fn := natsapi.Callee(info, n)
			if !natsapi.IsMethod(fn, natsapi.Core, "Header", "Set") && !natsapi.IsMethod(fn, natsapi.Core, "Header", "Add") {
				return true
			}
			sel := n.Fun.(*ast.SelectorExpr)
			if id, ok := headerOf(sel.X); ok && headerless(body, id) {
				report(n, "Header."+fn.Name())
			}
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				ix, ok := ast.Unparen(lhs).(*ast.IndexExpr)
				if !ok {
					continue
				}
				if id, ok := headerOf(ix.X); ok && headerless(body, id) {
					report(n, fmt.Sprintf("assignment to %s.Header", id.Name))
				}
			}
		}
		return true
	})
	return nil, nil
}

// assignsHeader reports whether body assigns obj.Header anywhere.
func assignsHeader(body *ast.BlockStmt, info *types.Info, obj types.Object) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || found {
			return !found
		}
		for _, lhs := range as.Lhs {
			sel, ok := ast.Unparen(lhs).(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Header" {
				continue
			}
			if id, ok := ast.Unparen(sel.X).(*ast.Ident); ok && info.ObjectOf(id) == obj {
				found = true
			}
		}
		return true
	})
	return found
}
