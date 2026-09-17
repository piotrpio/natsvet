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

import (
	"go/ast"
	"go/types"
	"strings"
)

// Discarded reports whether the call at the top of an inspector stack throws
// its first result away: it is an expression statement, or the single
// right-hand side of an assignment or var declaration whose first left-hand
// side is the blank identifier. go and defer statements, nested calls and
// returned calls are not discards.
func Discarded(stack []ast.Node) bool {
	if len(stack) < 2 {
		return false
	}
	call := stack[len(stack)-1]
	i := len(stack) - 2
	for ; i >= 0; i-- {
		if _, ok := stack[i].(*ast.ParenExpr); !ok {
			break
		}
	}
	if i < 0 {
		return false
	}
	switch p := stack[i].(type) {
	case *ast.ExprStmt:
		return true
	case *ast.AssignStmt:
		return len(p.Rhs) == 1 && ast.Unparen(p.Rhs[0]) == call && isBlank(p.Lhs[0])
	case *ast.ValueSpec:
		return len(p.Values) == 1 && ast.Unparen(p.Values[0]) == call && p.Names[0].Name == "_"
	}
	return false
}

func isBlank(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "_"
}

// ExitsProcess reports whether call never returns to the caller: the
// builtin panic, os.Exit, or a declared function or method whose name
// starts with Fatal (log.Fatal*, testing.T.Fatal*). A func-typed value is
// not examined, whatever it is named.
func ExitsProcess(info *types.Info, call *ast.CallExpr) bool {
	if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "panic" {
		if _, isBuiltin := info.Uses[id].(*types.Builtin); isBuiltin {
			return true
		}
	}
	fn := Callee(info, call)
	return IsFunc(fn, "os", "Exit") || fn != nil && strings.HasPrefix(fn.Name(), "Fatal")
}
