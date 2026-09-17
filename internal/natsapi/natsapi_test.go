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
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

// typecheck type-checks src as a single-file package with the given import
// path and returns the package, type info and file.
func typecheck(t *testing.T, path, src string) (*types.Package, *types.Info, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "src.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Uses:  make(map[*ast.Ident]types.Object),
		Defs:  make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{Importer: importer.Default()}
	pkg, err := conf.Check(path, fset, []*ast.File{f}, info)
	if err != nil {
		t.Fatal(err)
	}
	return pkg, info, f
}

// calls returns every call expression in f, in source order.
func calls(f *ast.File) []*ast.CallExpr {
	var out []*ast.CallExpr
	ast.Inspect(f, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			out = append(out, c)
		}
		return true
	})
	return out
}

func TestIsPkg(t *testing.T) {
	tests := []struct {
		name string
		path string
		pkg  Pkg
		want bool
	}{
		{"exact core", "github.com/nats-io/nats.go", Core, true},
		{"exact jetstream", "github.com/nats-io/nats.go/jetstream", JetStream, true},
		{"vendored", "example.com/app/vendor/github.com/nats-io/nats.go", Core, true},
		{"subpackage is not parent", "github.com/nats-io/nats.go/jetstream", Core, false},
		{"parent is not subpackage", "github.com/nats-io/nats.go", JetStream, false},
		{"unrelated", "example.com/nats", Core, false},
		{"suffix without vendor", "example.com/github.com/nats-io/nats.go", Core, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pkg, _, _ := typecheck(t, tt.path, "package p\ntype T struct{}")
			obj := pkg.Scope().Lookup("T")
			if got := IsPkg(obj, tt.pkg); got != tt.want {
				t.Errorf("IsPkg(%q, %q) = %v, want %v", tt.path, tt.pkg, got, tt.want)
			}
		})
	}
	t.Run("nil object", func(t *testing.T) {
		if IsPkg(nil, Core) {
			t.Error("IsPkg(nil) = true")
		}
	})
	t.Run("typed nil func", func(t *testing.T) {
		var fn *types.Func
		if IsPkg(fn, Core) {
			t.Error("IsPkg(typed nil) = true")
		}
	})
	t.Run("universe object", func(t *testing.T) {
		if IsPkg(types.Universe.Lookup("len"), Core) {
			t.Error("IsPkg(len) = true")
		}
	})
}

const methodSrc = `package p

type Conn struct{}

func (nc *Conn) Publish(subj string) error { return nil }

type Header map[string][]string

func (h Header) Get(key string) string { return "" }

type Msg interface {
	Headers() Header
}

type Base interface {
	Ack() error
}

type Extended interface {
	Base
}

func New() *Conn { return nil }

func use(nc *Conn, h Header, m Msg, e Extended, f func()) {
	nc.Publish("x")
	h.Get("k")
	m.Headers()
	e.Ack()
	New()
	f()
	_ = Header(nil)
	_ = len(h)
}
`

func TestCalleeAndIsMethod(t *testing.T) {
	_, info, f := typecheck(t, "github.com/nats-io/nats.go", methodSrc)
	var inUse []*ast.CallExpr
	for _, c := range calls(f) {
		if _, ok := info.Types[c.Fun]; ok {
			inUse = append(inUse, c)
		}
	}
	// Calls inside use(), in order.
	names := []string{"nc.Publish", "h.Get", "m.Headers", "e.Ack", "New", "f", "Header(nil)", "len"}
	if len(inUse) != len(names) {
		t.Fatalf("found %d calls, want %d", len(inUse), len(names))
	}
	tests := []struct {
		name     string
		wantFunc bool
		recv     string
		method   string
		isMethod bool
		isFunc   bool
	}{
		{"nc.Publish", true, "Conn", "Publish", true, false},
		{"h.Get", true, "Header", "Get", true, false},
		{"m.Headers", true, "Msg", "Headers", true, false},
		{"e.Ack", true, "Base", "Ack", true, false},
		{"New", true, "", "New", false, true},
		{"f", false, "", "", false, false},
		{"Header(nil)", false, "", "", false, false},
		{"len", false, "", "", false, false},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := Callee(info, inUse[i])
			if (fn != nil) != tt.wantFunc {
				t.Fatalf("Callee = %v, want func: %v", fn, tt.wantFunc)
			}
			if fn == nil {
				return
			}
			if got := IsMethod(fn, Core, tt.recv, tt.method); got != tt.isMethod {
				t.Errorf("IsMethod(%s.%s) = %v, want %v", tt.recv, tt.method, got, tt.isMethod)
			}
			if got := IsFunc(fn, Core, tt.method); got != tt.isFunc {
				t.Errorf("IsFunc(%s) = %v, want %v", tt.method, got, tt.isFunc)
			}
			if IsMethod(fn, JetStream, tt.recv, tt.method) {
				t.Errorf("IsMethod matched the wrong package")
			}
			if tt.isMethod && IsMethod(fn, Core, tt.recv, "Other") {
				t.Errorf("IsMethod matched the wrong method name")
			}
			if tt.isMethod && IsMethod(fn, Core, "Other", tt.method) {
				t.Errorf("IsMethod matched the wrong receiver")
			}
		})
	}
}

func TestConstString(t *testing.T) {
	src := `package p

const prefix = "orders"
const key = prefix + ".new"

var dyn string

func use() {
	_ = "lit"
	_ = key
	_ = prefix + ".x"
	_ = dyn
	_ = 5
}
`
	_, info, f := typecheck(t, "p", src)
	var exprs []ast.Expr
	for _, s := range f.Decls[len(f.Decls)-1].(*ast.FuncDecl).Body.List {
		exprs = append(exprs, s.(*ast.AssignStmt).Rhs[0])
	}
	tests := []struct {
		name string
		want string
		ok   bool
	}{
		{"literal", "lit", true},
		{"named constant", "orders.new", true},
		{"constant concatenation", "orders.x", true},
		{"variable", "", false},
		{"non-string constant", "", false},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ConstString(info, exprs[i])
			if got != tt.want || ok != tt.ok {
				t.Errorf("ConstString = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

const namedTypeSrc = `package p

type StopAfter int

var (
	a StopAfter
	p *StopAfter
	n = 10
)
`

func TestIsNamedType(t *testing.T) {
	pkg, _, _ := typecheck(t, string(JetStream), namedTypeSrc)
	other, _, _ := typecheck(t, "example.com/other", namedTypeSrc)
	lookup := func(p *types.Package, name string) types.Type { return p.Scope().Lookup(name).Type() }
	tests := []struct {
		name string
		t    types.Type
		want bool
	}{
		{"named", lookup(pkg, "a"), true},
		{"pointer", lookup(pkg, "p"), false},
		{"other package", lookup(other, "a"), false},
		{"untyped constant", types.Typ[types.UntypedInt], false},
		{"int variable", lookup(pkg, "n"), false},
	}
	for _, tt := range tests {
		if got := IsNamedType(tt.t, JetStream, "StopAfter"); got != tt.want {
			t.Errorf("%s: IsNamedType = %v, want %v", tt.name, got, tt.want)
		}
	}
}
