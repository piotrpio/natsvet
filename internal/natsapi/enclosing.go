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

// AssignsField reports whether any assignment statement in body writes to
// a selector named field (x.field = ..., x.field += ...), on any receiver.
func AssignsField(body *ast.BlockStmt, field string) bool {
	if body == nil {
		return false
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if found {
			return false
		}
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, lhs := range as.Lhs {
			if sel, ok := ast.Unparen(lhs).(*ast.SelectorExpr); ok && sel.Sel.Name == field {
				found = true
				return false
			}
		}
		return true
	})
	return found
}
