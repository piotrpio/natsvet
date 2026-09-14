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
	"go/token"
	"go/types"
)

// IsDurationType reports whether t is exactly time.Duration.
func IsDurationType(t types.Type) bool {
	n, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := n.Obj()
	return obj.Name() == "Duration" && obj.Pkg() != nil && obj.Pkg().Path() == "time"
}

// StructField returns the field named name of the struct type behind t
// (looking through one pointer and the named type), or nil.
func StructField(t types.Type, name string) *types.Var {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return nil
	}
	for i := range st.NumFields() {
		if f := st.Field(i); f.Name() == name {
			return f
		}
	}
	return nil
}

// SingleDefinition returns the expression a local variable is defined from
// when the enclosing function body assigns it exactly once: the
// right-hand side of its only :=, = or var statement, with a multi-value
// call counting as the definition of every variable on the left. It
// reports no definition for parameters and package-level variables, for a
// variable assigned more than once (a range clause counts), and for a
// variable whose address is taken anywhere in the body.
func SingleDefinition(info *types.Info, body *ast.BlockStmt, id *ast.Ident) (ast.Expr, bool) {
	obj, ok := info.ObjectOf(id).(*types.Var)
	if !ok || body == nil || obj.Pos() < body.Pos() || obj.Pos() >= body.End() {
		return nil, false
	}
	var (
		defs  int
		value ast.Expr
		addr  bool
	)
	isObj := func(e ast.Expr) bool {
		id, ok := ast.Unparen(e).(*ast.Ident)
		return ok && info.ObjectOf(id) == obj
	}
	record := func(lhs, rhs []ast.Expr) {
		for i, l := range lhs {
			if !isObj(l) {
				continue
			}
			defs++
			switch {
			case len(rhs) == len(lhs):
				value = rhs[i]
			case len(rhs) == 1:
				value = rhs[0]
			}
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			record(n.Lhs, n.Rhs)
		case *ast.ValueSpec:
			if len(n.Values) == 0 {
				return true
			}
			lhs := make([]ast.Expr, len(n.Names))
			for i, name := range n.Names {
				lhs[i] = name
			}
			record(lhs, n.Values)
		case *ast.RangeStmt:
			if n.Key != nil && isObj(n.Key) || n.Value != nil && isObj(n.Value) {
				defs++
				value = nil
			}
		case *ast.UnaryExpr:
			if n.Op == token.AND && isObj(n.X) {
				addr = true
			}
		}
		return true
	})
	if defs != 1 || value == nil || addr {
		return nil, false
	}
	return value, true
}
