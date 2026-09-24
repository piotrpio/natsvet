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
	"slices"
	"testing"
)

const litsSrc = `package p

type Source struct{ Name string }

type Config struct {
	Mirror  *Source
	Inline  Source
	Sources []*Source
	Values  []Source
}

var src = &Source{Name: "v"}

var (
	_ = Config{Mirror: &Source{Name: "m"}}
	_ = Config{Inline: Source{Name: "i"}}
	_ = Config{Sources: []*Source{{Name: "a"}, src}}
	_ = Config{Values: []Source{{Name: "b"}, {Name: "c"}}}
	_ = Config{Mirror: src}
	_ = Config{Mirror: nil}
	_ = Config{}
	_ = Config{Sources: []*Source{nil}}
	_ = Config{Sources: []*Source{&Source{Name: "d"}}}
	_ = Config{Mirror: (&Source{Name: "p"})}
	_ = Config{Mirror: &Source{Name: "masked"}}
)
`

func TestFieldsLits(t *testing.T) {
	_, info, f := typecheck(t, "p", litsSrc)
	var configs []*ast.CompositeLit
	ast.Inspect(f, func(n ast.Node) bool {
		if l, ok := n.(*ast.CompositeLit); ok {
			if named, ok := info.TypeOf(l).(*types.Named); ok && named.Obj().Name() == "Config" {
				configs = append(configs, l)
			}
		}
		return true
	})
	tests := []struct {
		name     string
		field    string
		assigned map[string]bool
		want     []string
		complete bool
	}{
		{"address-of literal", "Mirror", nil, []string{`"m"`}, true},
		{"struct literal", "Inline", nil, []string{`"i"`}, true},
		{"slice with a variable", "Sources", nil, []string{`"a"`}, false},
		{"value slice", "Values", nil, []string{`"b"`, `"c"`}, true},
		{"variable", "Mirror", nil, nil, false},
		{"nil", "Mirror", nil, nil, true},
		{"absent", "Mirror", nil, nil, true},
		{"nil element", "Sources", nil, nil, false},
		{"explicit address-of element", "Sources", nil, []string{`"d"`}, true},
		{"parenthesized", "Mirror", nil, []string{`"p"`}, true},
		{"masked", "Mirror", map[string]bool{"Mirror": true}, nil, false},
	}
	if len(configs) != len(tests) {
		t.Fatalf("found %d Config literals, want %d", len(configs), len(tests))
	}
	for i, tt := range tests {
		fields := make(map[string]ast.Expr)
		for _, elt := range configs[i].Elts {
			kv := elt.(*ast.KeyValueExpr)
			fields[kv.Key.(*ast.Ident).Name] = kv.Value
		}
		lits, complete := NewFields(info, fields, false, nil, tt.assigned).Lits(tt.field)
		var names []string
		for _, l := range lits {
			names = append(names, l.Elts[0].(*ast.KeyValueExpr).Value.(*ast.BasicLit).Value)
		}
		if !slices.Equal(names, tt.want) || complete != tt.complete {
			t.Errorf("%s: Lits(%q) = %v, %v; want %v, %v", tt.name, tt.field, names, complete, tt.want, tt.complete)
		}
	}
}
