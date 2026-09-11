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
	"go/constant"
	"go/types"
	"time"
)

// ConstInt returns the value of expr when it is an integer constant
// expression (including typed constants such as time.Duration).
func ConstInt(info *types.Info, expr ast.Expr) (int64, bool) {
	tv, ok := info.Types[expr]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.Int {
		return 0, false
	}
	return constant.Int64Val(tv.Value)
}

// ConstDuration is ConstInt for time.Duration-valued expressions.
func ConstDuration(info *types.Info, expr ast.Expr) (time.Duration, bool) {
	v, ok := ConstInt(info, expr)
	return time.Duration(v), ok
}

// ConstBool returns the value of expr when it is a boolean constant.
func ConstBool(info *types.Info, expr ast.Expr) (bool, bool) {
	tv, ok := info.Types[expr]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.Bool {
		return false, false
	}
	return constant.BoolVal(tv.Value), true
}

// SliceConstStrings returns the constant string elements of a slice
// literal, in order, and whether every element was constant. A nil literal
// yields no elements and complete; anything that is not a slice literal
// yields no elements and not complete.
func SliceConstStrings(info *types.Info, expr ast.Expr) (vals []string, complete bool) {
	switch e := ast.Unparen(expr).(type) {
	case *ast.Ident:
		if e.Name == "nil" && info.Types[e].IsNil() {
			return nil, true
		}
		return nil, false
	case *ast.CompositeLit:
		complete = true
		for _, elt := range e.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				elt = kv.Value
			}
			s, ok := ConstString(info, elt)
			if !ok {
				complete = false
				continue
			}
			vals = append(vals, s)
		}
		return vals, complete
	}
	return nil, false
}

// ConstEnum returns the name of the named constant expr denotes when that
// constant is declared in one of pkgs, so that nats.AckNonePolicy and
// jetstream.AckNonePolicy both report "AckNonePolicy".
func ConstEnum(info *types.Info, expr ast.Expr, pkgs ...Pkg) (string, bool) {
	var id *ast.Ident
	switch e := ast.Unparen(expr).(type) {
	case *ast.Ident:
		id = e
	case *ast.SelectorExpr:
		id = e.Sel
	default:
		return "", false
	}
	c, ok := info.Uses[id].(*types.Const)
	if !ok {
		return "", false
	}
	for _, p := range pkgs {
		if IsPkg(c, p) {
			return c.Name(), true
		}
	}
	return "", false
}
