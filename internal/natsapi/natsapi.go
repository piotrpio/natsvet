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

// Package natsapi holds the helpers natsvet rules share: recognizing nats.go
// packages, callees and methods by import path, reading constant arguments,
// and the tables of NATS header names and legacy JetStream symbols generated
// from the pinned nats.go.
//go:generate go run ../tablegen/cmd -testdata ../../testdata -out .

package natsapi

import (
	"go/ast"
	"go/constant"
	"go/types"
	"strings"

	"golang.org/x/tools/go/types/typeutil"
)

// Pkg is the import path of a package whose symbols rules match on.
type Pkg string

const (
	Core      Pkg = "github.com/nats-io/nats.go"
	JetStream Pkg = "github.com/nats-io/nats.go/jetstream"
	Micro     Pkg = "github.com/nats-io/nats.go/micro"
)

// IsPkg reports whether obj is declared in pkg, directly or through a
// vendor directory.
func IsPkg(obj types.Object, pkg Pkg) bool {
	if obj == nil || obj.Pkg() == nil {
		return false
	}
	return IsPkgPath(obj.Pkg().Path(), pkg)
}

// IsPkgPath reports whether path is pkg or a vendored copy of it.
func IsPkgPath(path string, pkg Pkg) bool {
	return path == string(pkg) || strings.HasSuffix(path, "/vendor/"+string(pkg))
}

// Callee returns the function or method that call invokes, or nil when the
// callee is not a declared function (a func-typed value, a builtin, a
// conversion).
func Callee(info *types.Info, call *ast.CallExpr) *types.Func {
	fn, _ := typeutil.Callee(info, call).(*types.Func)
	return fn
}

// IsMethod reports whether fn is the method recv.name declared in pkg. recv
// is the name of the receiver's named type or interface without package
// qualifier or pointer; for a method of an embedded interface it is the
// interface that declares the method.
func IsMethod(fn *types.Func, pkg Pkg, recv, name string) bool {
	if fn == nil || fn.Name() != name || !IsPkg(fn, pkg) {
		return false
	}
	r := fn.Signature().Recv()
	if r == nil {
		return false
	}
	return receiverName(r.Type()) == recv
}

// IsFunc reports whether fn is the package-level function pkg.name.
func IsFunc(fn *types.Func, pkg Pkg, name string) bool {
	return fn != nil && fn.Name() == name && IsPkg(fn, pkg) && fn.Signature().Recv() == nil
}

// receiverName returns the name of the named type behind a receiver type,
// looking through a pointer, or "" when there is none.
func receiverName(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := t.(*types.Named); ok {
		return n.Obj().Name()
	}
	return ""
}

// ConstString returns the value of expr when it is a constant string
// expression, resolving named constants and constant concatenations.
func ConstString(info *types.Info, expr ast.Expr) (string, bool) {
	tv, ok := info.Types[expr]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}
