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
	"testing"
)

const definitionSrc = `package p

import "time"

type Sub struct{}
type Conn struct{}

func (c *Conn) Subscribe(s string) (*Sub, error) { return nil, nil }

type Cfg struct {
	D time.Duration
	N int
}

type Alias time.Duration

func single(c *Conn) {
	sub, err := c.Subscribe("s")
	_, _ = sub, err
	sub.Ping()
}

type N int

func (N) Ping() {}

func singleValue(c *Conn) {
	x := N(5)
	x.Ping()
}

func varDecl(c *Conn) {
	var sub, _ = c.Subscribe("s")
	sub.Ping()
}

func twice(c *Conn) {
	sub, _ := c.Subscribe("s")
	sub, _ = c.Subscribe("t")
	sub.Ping()
}

func ranged(subs []*Sub) {
	for _, sub := range subs {
		sub.Ping()
	}
}

func param(sub *Sub) {
	sub.Ping()
}

var global, _ = (&Conn{}).Subscribe("g")

func pkgLevel() {
	global.Ping()
}

func addressTaken(c *Conn) {
	sub, _ := c.Subscribe("s")
	p := &sub
	_ = p
	sub.Ping()
}

func inClosure(c *Conn) {
	var sub *Sub
	f := func() { sub, _ = c.Subscribe("s") }
	f()
	sub.Ping()
}

func (s *Sub) Ping() {}
`

func TestSingleDefinition(t *testing.T) {
	_, info, f := typecheck(t, "p", definitionSrc)
	tests := []struct {
		fn   string
		want string // "" for no definition; otherwise a description of the expression
	}{
		{"single", "call"},
		{"singleValue", "lit"},
		{"varDecl", "call"},
		{"twice", ""},
		{"ranged", ""},
		{"param", ""},
		{"pkgLevel", ""},
		{"addressTaken", ""},
		{"inClosure", "call"},
	}
	for _, tt := range tests {
		fd := funcDecl(f, tt.fn)
		id := pingReceiver(fd)
		got, ok := SingleDefinition(info, fd.Body, id)
		switch {
		case tt.want == "" && ok:
			t.Errorf("%s: got definition %T, want none", tt.fn, got)
		case tt.want == "call":
			if _, isCall := got.(*ast.CallExpr); !ok || !isCall {
				t.Errorf("%s: got (%T, %v), want a call", tt.fn, got, ok)
			}
		case tt.want == "lit":
			if _, isConv := got.(*ast.CallExpr); !ok || !isConv {
				t.Errorf("%s: got (%T, %v), want the conversion", tt.fn, got, ok)
			}
		}
	}
}

func funcDecl(f *ast.File, name string) *ast.FuncDecl {
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == name {
			return fd
		}
	}
	return nil
}

// pingReceiver returns the receiver identifier of the x.Ping() call in fd.
func pingReceiver(fd *ast.FuncDecl) *ast.Ident {
	var id *ast.Ident
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Ping" {
			id, _ = sel.X.(*ast.Ident)
		}
		return true
	})
	return id
}

func TestIsDurationTypeAndStructField(t *testing.T) {
	pkg, _, _ := typecheck(t, "p", definitionSrc)
	cfg := pkg.Scope().Lookup("Cfg").Type()
	if f := StructField(cfg, "D"); f == nil || !IsDurationType(f.Type()) {
		t.Errorf("Cfg.D: field %v, IsDurationType false", f)
	}
	if f := StructField(types.NewPointer(cfg), "N"); f == nil || IsDurationType(f.Type()) {
		t.Errorf("*Cfg.N: field %v, IsDurationType true", f)
	}
	if StructField(cfg, "Missing") != nil {
		t.Error("missing field found")
	}
	if StructField(types.Typ[types.Int], "D") != nil {
		t.Error("field found on a non-struct")
	}
	if IsDurationType(pkg.Scope().Lookup("Alias").Type()) {
		t.Error("named type with Duration underlying reported as Duration")
	}
	if IsDurationType(types.Typ[types.Int64]) {
		t.Error("int64 reported as Duration")
	}
}
