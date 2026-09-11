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
)

const enclosingSrc = `package p

type cfg struct{ DeliverSubject, Other string; N int }

func direct() cfg {
	c := cfg{Other: "x"}
	c.DeliverSubject = "d"
	c.N++
	return c
}

func nested() {
	f := func() {
		var c cfg
		(c.DeliverSubject) = "d"
	}
	f()
}

func untouched() cfg {
	c := cfg{Other: "x", N: 7}
	c.Other = "y"
	return c
}

var pkgLevel = cfg{Other: "z"}
`

func funcBody(f *ast.File, name string) *ast.BlockStmt {
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == name {
			return fd.Body
		}
	}
	return nil
}

func TestAssignedFields(t *testing.T) {
	_, _, f := typecheck(t, "p", enclosingSrc)
	tests := []struct {
		fn   string
		want []string
	}{
		{"direct", []string{"DeliverSubject", "N"}},
		{"nested", []string{"DeliverSubject"}},
		{"untouched", []string{"Other"}},
	}
	for _, tt := range tests {
		got := AssignedFields(funcBody(f, tt.fn))
		if len(got) != len(tt.want) {
			t.Errorf("AssignedFields(%s) = %v, want %v", tt.fn, got, tt.want)
		}
		for _, w := range tt.want {
			if !got[w] {
				t.Errorf("AssignedFields(%s) lacks %s", tt.fn, w)
			}
		}
	}
	if AssignedFields(nil) != nil {
		t.Error("AssignedFields(nil) != nil")
	}
}

func TestEnclosingFuncBody(t *testing.T) {
	_, _, f := typecheck(t, "p", enclosingSrc)
	var stack []ast.Node
	var lits []*ast.BlockStmt
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		stack = append(stack, n)
		if _, ok := n.(*ast.CompositeLit); ok {
			lits = append(lits, EnclosingFuncBody(stack))
		}
		return true
	})
	if len(lits) != 3 {
		t.Fatalf("found %d literals", len(lits))
	}
	if lits[0] == nil || lits[1] == nil {
		t.Error("literal inside a function has no enclosing body")
	}
	if lits[2] != nil {
		t.Error("package-level literal has an enclosing body")
	}
}

// TestFieldsAssignedUnknown checks that a field the function assigns is
// unknown to every accessor, present in the literal or not.
func TestFieldsAssignedUnknown(t *testing.T) {
	_, info, f := typecheck(t, "p", enclosingSrc)
	body := funcBody(f, "untouched")
	var lit *ast.CompositeLit
	ast.Inspect(body, func(n ast.Node) bool {
		if l, ok := n.(*ast.CompositeLit); ok && lit == nil {
			lit = l
		}
		return true
	})
	fields, _, ok := CompositeFields(info, lit, TypeRef{"p", "cfg"})
	if !ok {
		t.Fatal("literal not matched")
	}
	fs := NewFields(info, fields, false, nil, AssignedFields(body))

	if _, ok := fs.Str("Other"); ok {
		t.Error("assigned present field reported as known")
	}
	if v, ok := fs.Int("N"); !ok || v != 7 {
		t.Errorf("unassigned present field = (%v, %v), want (7, true)", v, ok)
	}
	if v, ok := fs.Str("DeliverSubject"); !ok || v != "" {
		t.Errorf("unassigned absent field = (%q, %v), want (\"\", true)", v, ok)
	}

	// With the alias, the assignment to the legacy name masks the canonical one.
	fs = NewFields(info, fields, true, map[string]string{"Canonical": "Other"}, AssignedFields(body))
	if _, ok := fs.Str("Canonical"); ok {
		t.Error("aliased assigned field reported as known")
	}
}
