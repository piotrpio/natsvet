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
)

// TypeRef names a struct type of a nats.go package.
type TypeRef struct {
	Pkg  Pkg
	Name string
}

// CompositeFields returns the keyed fields of lit by name when lit's type
// (looking through one pointer, as for elements of []*T{{...}}) is one of
// refs, along with the ref that matched. A positional literal or a literal
// of another type yields ok == false.
func CompositeFields(info *types.Info, lit *ast.CompositeLit, refs ...TypeRef) (fields map[string]ast.Expr, matched TypeRef, ok bool) {
	t := info.TypeOf(lit)
	if p, isPtr := t.(*types.Pointer); isPtr {
		t = p.Elem()
	}
	n, isNamed := t.(*types.Named)
	if !isNamed {
		return nil, TypeRef{}, false
	}
	for _, r := range refs {
		if n.Obj().Name() == r.Name && IsPkg(n.Obj(), r.Pkg) {
			matched = r
			ok = true
			break
		}
	}
	if !ok {
		return nil, TypeRef{}, false
	}
	fields = make(map[string]ast.Expr, len(lit.Elts))
	for _, elt := range lit.Elts {
		kv, keyed := elt.(*ast.KeyValueExpr)
		if !keyed {
			return nil, TypeRef{}, false
		}
		if id, isIdent := kv.Key.(*ast.Ident); isIdent {
			fields[id.Name] = kv.Value
		}
	}
	return fields, matched, true
}
