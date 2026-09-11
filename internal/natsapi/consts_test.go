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
	"testing"
	"time"
)

const constsSrc = `package p

import "time"

type policy int

const (
	none policy = iota
	explicit
)

const prefix = "orders"

var dyn string
var dynInts []string
var dynN int

type cfg struct {
	d    time.Duration
	n    int
	b    bool
	s    []string
	p    policy
}

var _ = []cfg{
	{d: 50 * time.Millisecond},
	{d: 30},
	{d: -time.Second},
	{d: time.Duration(dynN)},
	{n: 5},
	{n: 2 + 3},
	{n: dynN},
	{b: true},
	{b: !false},
	{b: dynN > 0},
	{s: []string{"a", prefix + ".b"}},
	{s: []string{"a", dyn}},
	{s: nil},
	{s: dynInts},
	{s: []string{}},
	{p: explicit},
	{p: 0},
	{p: policy(dynN)},
}
`

// fieldValues returns the value expression of the single keyed field of
// each element literal in the last var declaration of src.
func fieldValues(t *testing.T, f *ast.File) []ast.Expr {
	t.Helper()
	decl := f.Decls[len(f.Decls)-1].(*ast.GenDecl)
	lit := decl.Specs[0].(*ast.ValueSpec).Values[0].(*ast.CompositeLit)
	var out []ast.Expr
	for _, elt := range lit.Elts {
		out = append(out, elt.(*ast.CompositeLit).Elts[0].(*ast.KeyValueExpr).Value)
	}
	return out
}

func TestConstExtractors(t *testing.T) {
	_, info, f := typecheck(t, string(Core), constsSrc)
	v := fieldValues(t, f)
	if len(v) != 18 {
		t.Fatalf("got %d values", len(v))
	}

	t.Run("duration", func(t *testing.T) {
		tests := []struct {
			want time.Duration
			ok   bool
		}{{50 * time.Millisecond, true}, {30, true}, {-time.Second, true}, {0, false}}
		for i, tt := range tests {
			got, ok := ConstDuration(info, v[i])
			if got != tt.want || ok != tt.ok {
				t.Errorf("%d: ConstDuration = (%v, %v), want (%v, %v)", i, got, ok, tt.want, tt.ok)
			}
		}
	})
	t.Run("int", func(t *testing.T) {
		tests := []struct {
			want int64
			ok   bool
		}{{5, true}, {5, true}, {0, false}}
		for i, tt := range tests {
			got, ok := ConstInt(info, v[4+i])
			if got != tt.want || ok != tt.ok {
				t.Errorf("%d: ConstInt = (%v, %v), want (%v, %v)", i, got, ok, tt.want, tt.ok)
			}
		}
	})
	t.Run("bool", func(t *testing.T) {
		tests := []struct{ want, ok bool }{{true, true}, {true, true}, {false, false}}
		for i, tt := range tests {
			got, ok := ConstBool(info, v[7+i])
			if got != tt.want || ok != tt.ok {
				t.Errorf("%d: ConstBool = (%v, %v), want (%v, %v)", i, got, ok, tt.want, tt.ok)
			}
		}
	})
	t.Run("slice", func(t *testing.T) {
		tests := []struct {
			want     []string
			complete bool
		}{
			{[]string{"a", "orders.b"}, true},
			{[]string{"a"}, false},
			{nil, true},
			{nil, false},
			{nil, true},
		}
		for i, tt := range tests {
			got, complete := SliceConstStrings(info, v[10+i])
			if complete != tt.complete || len(got) != len(tt.want) {
				t.Fatalf("%d: SliceConstStrings = (%v, %v), want (%v, %v)", i, got, complete, tt.want, tt.complete)
			}
			for j := range got {
				if got[j] != tt.want[j] {
					t.Errorf("%d: element %d = %q, want %q", i, j, got[j], tt.want[j])
				}
			}
		}
	})
	t.Run("enum", func(t *testing.T) {
		tests := []struct {
			want string
			ok   bool
		}{{"explicit", true}, {"", false}, {"", false}}
		for i, tt := range tests {
			got, ok := ConstEnum(info, v[15+i], Core, JetStream)
			if got != tt.want || ok != tt.ok {
				t.Errorf("%d: ConstEnum = (%q, %v), want (%q, %v)", i, got, ok, tt.want, tt.ok)
			}
		}
		if _, ok := ConstEnum(info, v[15], JetStream); ok {
			t.Error("ConstEnum matched a constant from a package not in the list")
		}
	})
}

func TestConstEnumAcrossPackages(t *testing.T) {
	src := "package p\n\ntype AckPolicy int\n\nconst AckNonePolicy AckPolicy = 2\n\nvar _ = AckNonePolicy\n"
	for _, pkg := range []Pkg{Core, JetStream} {
		_, info, f := typecheck(t, string(pkg), src)
		expr := f.Decls[len(f.Decls)-1].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Values[0]
		name, ok := ConstEnum(info, expr, Core, JetStream)
		if !ok || name != "AckNonePolicy" {
			t.Errorf("%s: ConstEnum = (%q, %v)", pkg, name, ok)
		}
	}
	_, info, f := typecheck(t, "example.com/user", src)
	expr := f.Decls[len(f.Decls)-1].(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Values[0]
	if _, ok := ConstEnum(info, expr, Core, JetStream); ok {
		t.Error("user constant with the same name matched")
	}
}
