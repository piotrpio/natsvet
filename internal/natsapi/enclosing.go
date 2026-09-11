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

package natsapi

import "go/ast"

// EnclosingFuncBody returns the body of the innermost function declaration
// or literal in stack (as produced by inspector.WithStack), or nil at
// package level.
func EnclosingFuncBody(stack []ast.Node) *ast.BlockStmt {
	for i := len(stack) - 1; i >= 0; i-- {
		switch n := stack[i].(type) {
		case *ast.FuncDecl:
			return n.Body
		case *ast.FuncLit:
			return n.Body
		}
	}
	return nil
}

// AssignedFields returns the names of every selector that an assignment or
// increment statement in body writes to (x.Field = ..., x.Field += ...,
// x.Field++), on any receiver. A config literal in such a function may be
// completed or changed after it is written, so those fields are unknown
// for every literal in the function.
func AssignedFields(body *ast.BlockStmt) map[string]bool {
	if body == nil {
		return nil
	}
	fields := make(map[string]bool)
	add := func(e ast.Expr) {
		if sel, ok := ast.Unparen(e).(*ast.SelectorExpr); ok {
			fields[sel.Sel.Name] = true
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range n.Lhs {
				add(lhs)
			}
		case *ast.IncDecStmt:
			add(n.X)
		}
		return true
	})
	return fields
}
