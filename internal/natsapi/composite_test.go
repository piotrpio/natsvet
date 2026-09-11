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

const compositeSrc = `package p

type StreamConfig struct {
	Name     string
	Replicas int
}

type ConsumerConfig struct {
	Durable string
}

type Other struct {
	Name string
}

var (
	_ = StreamConfig{Name: "a", Replicas: 3}
	_ = &StreamConfig{Name: "b"}
	_ = []StreamConfig{{Name: "c"}}
	_ = []*StreamConfig{{Name: "d"}}
	_ = map[string]StreamConfig{"k": {Name: "e"}}
	_ = StreamConfig{"pos", 1}
	_ = ConsumerConfig{Durable: "f"}
	_ = Other{Name: "g"}
	_ = StreamConfig{}
)
`

func TestCompositeFields(t *testing.T) {
	_, info, f := typecheck(t, string(JetStream), compositeSrc)
	var lits []*ast.CompositeLit
	ast.Inspect(f, func(n ast.Node) bool {
		if l, ok := n.(*ast.CompositeLit); ok && !isContainerLit(info, l) {
			lits = append(lits, l)
		}
		return true
	})
	refs := []TypeRef{{JetStream, "StreamConfig"}, {Core, "StreamConfig"}, {JetStream, "ConsumerConfig"}}
	tests := []struct {
		name    string
		ok      bool
		matched TypeRef
		field   string
		value   string
	}{
		{"value", true, TypeRef{JetStream, "StreamConfig"}, "Name", `"a"`},
		{"address-of", true, TypeRef{JetStream, "StreamConfig"}, "Name", `"b"`},
		{"slice element", true, TypeRef{JetStream, "StreamConfig"}, "Name", `"c"`},
		{"pointer slice element", true, TypeRef{JetStream, "StreamConfig"}, "Name", `"d"`},
		{"map element", true, TypeRef{JetStream, "StreamConfig"}, "Name", `"e"`},
		{"positional", false, TypeRef{}, "", ""},
		{"other accepted type", true, TypeRef{JetStream, "ConsumerConfig"}, "Durable", `"f"`},
		{"unrelated type", false, TypeRef{}, "", ""},
		{"empty", true, TypeRef{JetStream, "StreamConfig"}, "", ""},
	}
	if len(lits) != len(tests) {
		t.Fatalf("found %d literals, want %d", len(lits), len(tests))
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fields, matched, ok := CompositeFields(info, lits[i], refs...)
			if ok != tt.ok || matched != tt.matched {
				t.Fatalf("CompositeFields = (%v, %v, %v), want (_, %v, %v)", fields, matched, ok, tt.matched, tt.ok)
			}
			if tt.field == "" {
				return
			}
			lit, isLit := fields[tt.field].(*ast.BasicLit)
			if !isLit || lit.Value != tt.value {
				t.Errorf("field %s = %v, want %s", tt.field, fields[tt.field], tt.value)
			}
		})
	}
	if _, _, ok := CompositeFields(info, lits[0], TypeRef{Core, "StreamConfig"}); ok {
		t.Error("matched a type from the wrong package")
	}
}

// isContainerLit reports whether l is a slice or map literal rather than a
// struct literal.
func isContainerLit(info *types.Info, l *ast.CompositeLit) bool {
	switch info.TypeOf(l).Underlying().(type) {
	case *types.Slice, *types.Map, *types.Array:
		return true
	}
	return false
}
