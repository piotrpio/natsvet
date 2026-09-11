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

type cfg struct{ DeliverSubject, Other string }

func direct() cfg {
	c := cfg{Other: "x"}
	c.DeliverSubject = "d"
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
	c := cfg{Other: "x"}
	c.Other = "y"
	return c
}

var pkgLevel = cfg{Other: "z"}
`

func TestAssignsField(t *testing.T) {
	_, _, f := typecheck(t, "p", enclosingSrc)
	tests := []struct {
		fn   string
		want bool
	}{{"direct", true}, {"nested", true}, {"untouched", false}}
	for _, tt := range tests {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Name.Name != tt.fn {
				continue
			}
			if got := AssignsField(fd.Body, "DeliverSubject"); got != tt.want {
				t.Errorf("AssignsField(%s) = %v, want %v", tt.fn, got, tt.want)
			}
		}
	}
	if AssignsField(nil, "DeliverSubject") {
		t.Error("AssignsField(nil) = true")
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
