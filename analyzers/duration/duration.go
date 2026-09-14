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

// Package duration reports untyped integer constants passed where nats.go
// expects a time.Duration.
package duration

import (
	"fmt"
	"go/ast"
	"go/types"
	"time"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

const doc = `duration: report untyped constants passed where nats.go expects a time.Duration

Go converts an untyped integer constant to time.Duration silently, so
nc.Request("s", nil, 5) waits five nanoseconds and AckWait: 30 acknowledges
in thirty nanoseconds; the intended unit was almost certainly seconds or
milliseconds. Arguments, struct literal fields, field assignments and
conversions to nats.go's Duration-based option types are covered, for every
API in the nats, jetstream and micro packages.

	nc.Request("s", nil, 5)            // 5ns
	nc.Request("s", nil, 5*time.Second)`

const name = "duration"

var Analyzer = &analysis.Analyzer{
	Name:     name,
	Doc:      doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

var natsPkgs = []natsapi.Pkg{natsapi.Core, natsapi.JetStream, natsapi.Micro}

func inNats(obj types.Object) bool {
	for _, p := range natsPkgs {
		if natsapi.IsPkg(obj, p) {
			return true
		}
	}
	return false
}

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	info := pass.TypesInfo
	check := func(e ast.Expr, where string) {
		v, ok := untypedConst(info, e)
		if !ok || v <= 0 {
			return
		}
		pass.Report(analysis.Diagnostic{
			Pos:      e.Pos(),
			End:      e.End(),
			Category: name,
			Message: fmt.Sprintf("duration %d for %s is %s; multiply by a time unit such as time.Second or time.Millisecond",
				v, where, time.Duration(v)),
		})
	}
	// checkValue applies check to e when t is a Duration, or to each
	// element of a slice literal when t is []Duration.
	checkValue := func(e ast.Expr, t types.Type, where string) {
		switch {
		case natsapi.IsDurationType(t):
			check(e, where)
		case isDurationSlice(t):
			if lit, ok := ast.Unparen(e).(*ast.CompositeLit); ok {
				for _, elt := range lit.Elts {
					check(elt, where)
				}
			}
		}
	}
	ins.Preorder([]ast.Node{(*ast.CallExpr)(nil), (*ast.CompositeLit)(nil), (*ast.AssignStmt)(nil)}, func(n ast.Node) {
		switch n := n.(type) {
		case *ast.CallExpr:
			if fn := natsapi.Callee(info, n); fn != nil {
				if !inNats(fn) {
					return
				}
				sig := fn.Signature()
				params := sig.Params()
				for i, a := range n.Args {
					var pt types.Type
					switch {
					case sig.Variadic() && i >= params.Len()-1:
						pt = params.At(params.Len() - 1).Type().(*types.Slice).Elem()
					case i < params.Len():
						pt = params.At(i).Type()
					default:
						continue
					}
					checkValue(a, pt, fn.Name())
				}
				return
			}
			// A conversion to a nats.go type declared as time.Duration.
			if tv, ok := info.Types[n.Fun]; ok && tv.IsType() && len(n.Args) == 1 {
				if named, ok := tv.Type.(*types.Named); ok && natsapi.IsDurationAlias(named.Obj()) {
					check(n.Args[0], named.Obj().Name())
				}
			}
		case *ast.CompositeLit:
			t := info.TypeOf(n)
			if p, ok := t.(*types.Pointer); ok {
				t = p.Elem()
			}
			named, ok := t.(*types.Named)
			if !ok || !inNats(named.Obj()) {
				return
			}
			for _, elt := range n.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := kv.Key.(*ast.Ident)
				if !ok {
					continue
				}
				if f := natsapi.StructField(named, key.Name); f != nil {
					checkValue(kv.Value, f.Type(), key.Name)
				}
			}
		case *ast.AssignStmt:
			if len(n.Lhs) != len(n.Rhs) {
				return
			}
			for i, lhs := range n.Lhs {
				sel, ok := ast.Unparen(lhs).(*ast.SelectorExpr)
				if !ok {
					continue
				}
				f, ok := info.Uses[sel.Sel].(*types.Var)
				if !ok || !f.IsField() || !inNats(f) {
					continue
				}
				checkValue(n.Rhs[i], f.Type(), f.Name())
			}
		}
	})
	return nil, nil
}

func isDurationSlice(t types.Type) bool {
	s, ok := t.(*types.Slice)
	return ok && natsapi.IsDurationType(s.Elem())
}

// untypedConst returns the value of e when it is an integer constant
// expression built only from literals, untyped constants and arithmetic:
// no typed constant such as time.Second, no conversion, no call.
func untypedConst(info *types.Info, e ast.Expr) (int64, bool) {
	if !isUntypedConstExpr(info, e) {
		return 0, false
	}
	return natsapi.ConstInt(info, e)
}

func isUntypedConstExpr(info *types.Info, e ast.Expr) bool {
	switch e := e.(type) {
	case *ast.BasicLit:
		return true
	case *ast.ParenExpr:
		return isUntypedConstExpr(info, e.X)
	case *ast.UnaryExpr:
		return isUntypedConstExpr(info, e.X)
	case *ast.BinaryExpr:
		return isUntypedConstExpr(info, e.X) && isUntypedConstExpr(info, e.Y)
	case *ast.Ident:
		return isUntypedConstObj(info.Uses[e])
	case *ast.SelectorExpr:
		return isUntypedConstObj(info.Uses[e.Sel])
	}
	return false
}

func isUntypedConstObj(obj types.Object) bool {
	c, ok := obj.(*types.Const)
	if !ok {
		return false
	}
	b, ok := c.Type().(*types.Basic)
	return ok && b.Info()&types.IsUntyped != 0
}
