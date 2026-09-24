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
	"sort"
	"strings"
	"testing"
)

// maskerSrc is type-checked with the Core import path, so js.Submit counts
// as a nats.go method and fmt.Println as an unrelated call.
const maskerSrc = `package nats

import "fmt"

type Config struct {
	Name           string
	DeliverSubject string
	Heartbeat      int
	Subjects       []string
	Meta           map[string]string
	Grid           [][]string
}

type JS struct{}

func (JS) Submit(c *Config) error  { return nil }
func (JS) SubmitV(c Config) error  { return nil }
func (c *Config) Reset()           {}

func createCons(c Config)     {}

func inline(js JS) {
	js.SubmitV(Config{Name: "a.b"})
	var other Config
	other.Name = "x"
}

func returned() Config {
	return Config{Name: "a.b"}
}

func usedBeforeMutation(js JS) {
	cfg := Config{Heartbeat: 5}
	js.SubmitV(cfg)
	cfg.DeliverSubject = "d"
	js.SubmitV(cfg)
}

func wrapperByValue() {
	cfg := Config{Heartbeat: 5}
	createCons(cfg)
	cfg.DeliverSubject = "d"
	createCons(cfg)
}

func mutatedBeforeUse(js JS) {
	cfg := Config{Heartbeat: 5}
	cfg.DeliverSubject = "d"
	js.SubmitV(cfg)
	cfg.Name = "n"
}

func conditionalBeforeUse(js JS, push bool) {
	cfg := Config{Heartbeat: 5}
	if push {
		cfg.DeliverSubject = "d"
	}
	js.SubmitV(cfg)
	cfg.Name = "n"
}

func pointerToNats(js JS) {
	cfg := Config{Heartbeat: 5}
	js.Submit(&cfg)
	cfg.DeliverSubject = "d"
}

func pointerToWrapper() {
	cfg := Config{Heartbeat: 5}
	fmt.Println(&cfg)
	cfg.DeliverSubject = "d"
}

func pointerVar(js JS) {
	cfg := &Config{Heartbeat: 5}
	js.Submit(cfg)
	cfg.DeliverSubject = "d"
}

func pointerVarToWrapper() {
	cfg := &Config{Heartbeat: 5}
	fmt.Println(cfg)
	cfg.DeliverSubject = "d"
}

func aliased(js JS) {
	cfg := Config{Heartbeat: 5}
	p := &cfg
	js.SubmitV(cfg)
	p.DeliverSubject = "d"
}

func methodCall(js JS) {
	cfg := Config{Heartbeat: 5}
	cfg.Reset()
	js.SubmitV(cfg)
	cfg.Name = "n"
}

func captured(js JS) {
	cfg := Config{Heartbeat: 5}
	f := func() { cfg.DeliverSubject = "d" }
	f()
	js.SubmitV(cfg)
}

func table(js JS) {
	tests := []struct{ cc *Config }{{cc: &Config{Heartbeat: 5}}}
	for _, test := range tests {
		cc := test.cc
		cc.DeliverSubject = "d"
		js.Submit(cc)
	}
}

func neverUsed() {
	cfg := Config{Name: "a.b"}
	_ = cfg
}

func varDecl(js JS) {
	var cfg = Config{Heartbeat: 5}
	js.SubmitV(cfg)
	cfg.DeliverSubject = "d"
}

func printed(js JS) {
	cfg := Config{Heartbeat: 5}
	fmt.Println(cfg)
	cfg.DeliverSubject = "d"
	js.SubmitV(cfg)
}

func funcValue(js JS) {
	cfg := Config{Heartbeat: 5}
	f := func(c *Config) {}
	f(&cfg)
	cfg.DeliverSubject = "d"
}

func loggedThenFixed(js JS) {
	cfg := Config{Heartbeat: 5}
	fmt.Printf("creating %v", cfg)
	cfg.DeliverSubject = "d"
	js.SubmitV(cfg)
}

func genericByValue(js JS) {
	cfg := Config{Heartbeat: 5}
	use(cfg)
	cfg.DeliverSubject = "d"
}

func use[T any](v T) {}

func storedInField(js JS) {
	var h struct{ c Config }
	h.c = Config{Heartbeat: 5}
	h.c.DeliverSubject = "d"
	js.SubmitV(h.c)
}

func indexedBeforeUse(js JS) {
	cfg := Config{Subjects: []string{"a", "a"}}
	cfg.Subjects[1] = "b"
	js.SubmitV(cfg)
}

func indexChainBeforeUse(js JS) {
	cfg := Config{Name: "a"}
	cfg.Grid[0][1] = "x"
	cfg.Meta["k"] = "v"
	js.SubmitV(cfg)
}

var pkgLevel = Config{Name: "a.b"}
`

func TestMasker(t *testing.T) {
	_, info, f := typecheck(t, string(Core), maskerSrc)
	tests := map[string]string{
		"inline":               "-", // temporary: nothing masked
		"returned":             "-", // temporary
		"usedBeforeMutation":   "",  // handed off before any assignment
		"wrapperByValue":       "",  // by-value hand-off to any call counts
		"mutatedBeforeUse":     "DeliverSubject",
		"conditionalBeforeUse": "DeliverSubject",
		"pointerToNats":        "",               // pointer to a nats.go method is a hand-off
		"pointerToWrapper":     "DeliverSubject", // pointer to a non-nats call: whole function
		"pointerVar":           "",
		"pointerVarToWrapper":  "DeliverSubject",
		"aliased":              "DeliverSubject", // &cfg escapes before the hand-off
		"methodCall":           "Name",           // receiver: whole function
		"captured":             "DeliverSubject", // closure: whole function
		"table":                "DeliverSubject", // no hand-off of tests: whole function
		"neverUsed":            "",               // whole function, nothing assigned
		"varDecl":              "",
		"printed":              "DeliverSubject",   // fmt.Println takes any: not a hand-off
		"funcValue":            "DeliverSubject",   // call through a func value: pointer escape
		"loggedThenFixed":      "DeliverSubject",   // the logger takes any; the submission comes after the fix
		"genericByValue":       "",                 // instantiated generic parameter is concretely typed
		"storedInField":        "DeliverSubject,c", // not bound to an identifier: whole function (h.c counts)
		"indexedBeforeUse":     "Subjects",         // an element assignment changes the field
		"indexChainBeforeUse":  "Grid,Meta",
		"pkgLevel":             "-", // no enclosing function
	}
	var stack []ast.Node
	seen := make(map[string]bool)
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		stack = append(stack, n)
		lit, ok := n.(*ast.CompositeLit)
		if !ok || !isConfigLit(info, lit) {
			return true
		}
		fn := enclosingName(stack)
		want, known := tests[fn]
		if !known {
			t.Errorf("no expectation for literal in %s", fn)
			return true
		}
		seen[fn] = true
		got := NewMasker(info).Masked(stack)
		if want == "-" {
			if got != nil {
				t.Errorf("%s: masked = %v, want nil (temporary)", fn, got)
			}
			return true
		}
		var names []string
		for k := range got {
			names = append(names, k)
		}
		sort.Strings(names)
		if joined := strings.Join(names, ","); joined != want {
			t.Errorf("%s: masked = %q, want %q", fn, joined, want)
		}
		return true
	})
	for fn := range tests {
		if !seen[fn] {
			t.Errorf("no literal found in %s", fn)
		}
	}
}

func isConfigLit(info *types.Info, lit *ast.CompositeLit) bool {
	_, _, ok := CompositeFields(info, lit, TypeRef{Core, "Config"})
	return ok
}

func enclosingName(stack []ast.Node) string {
	for i := len(stack) - 1; i >= 0; i-- {
		if fd, ok := stack[i].(*ast.FuncDecl); ok {
			return fd.Name.Name
		}
	}
	for i := len(stack) - 1; i >= 0; i-- {
		if vs, ok := stack[i].(*ast.ValueSpec); ok {
			return vs.Names[0].Name
		}
	}
	return "?"
}
