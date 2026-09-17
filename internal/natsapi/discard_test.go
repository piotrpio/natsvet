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

	"golang.org/x/tools/go/ast/inspector"
)

const discardSrc = `package p

type T struct{}

func (T) Call() (int, error) { return 0, nil }
func (T) One() int           { return 0 }
func other(int)              {}

var t T

func exprStmt()   { t.Call() }
func blankDefine() { _, err := t.Call(); _ = err }
func blankAssign() {
	var err error
	_, err = t.Call()
	_ = err
}
func blankBoth()  { _, _ = t.Call() }
func parenthesized() { _, _ = (t.Call()) }
func ifInit() {
	if _, err := t.Call(); err != nil {
		return
	}
}
func forInit() {
	for _, err := t.Call(); err != nil; {
		break
	}
}
func switchInit() {
	switch _, err := t.Call(); err {
	case nil:
	}
}
func varBlank() { var _, _ = t.Call() }
func named()    { n, err := t.Call(); _, _ = n, err }
func goStmt()   { go t.Call() }
func deferStmt() { defer t.Call() }
func nested()   { other(t.One()) }
func returned() (int, error) { return t.Call() }
`

func TestDiscarded(t *testing.T) {
	_, _, f := typecheck(t, "p", discardSrc)
	want := map[string]bool{
		"exprStmt":      true,
		"blankDefine":   true,
		"blankAssign":   true,
		"blankBoth":     true,
		"parenthesized": true,
		"ifInit":        true,
		"forInit":       true,
		"switchInit":    true,
		"varBlank":      true,
		"named":         false,
		"goStmt":        false,
		"deferStmt":     false,
		"nested":        false,
		"returned":      false,
	}
	got := map[string]bool{}
	ins := inspector.New([]*ast.File{f})
	ins.WithStack([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return false
		}
		sel, ok := n.(*ast.CallExpr).Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "Call" && sel.Sel.Name != "One") {
			return true
		}
		for _, s := range stack {
			if fd, ok := s.(*ast.FuncDecl); ok {
				got[fd.Name.Name] = Discarded(stack)
			}
		}
		return true
	})
	for fn, w := range want {
		g, ok := got[fn]
		if !ok {
			t.Errorf("%s: no hooked call found", fn)
			continue
		}
		if g != w {
			t.Errorf("Discarded in %s = %v, want %v", fn, g, w)
		}
	}
}

const exitsSrc = `package p

import (
	"log"
	"os"
	"testing"
)

func exitCall()  { os.Exit(1) }
func fatalCall() { log.Fatalf("x") }
func testFatal(t *testing.T) { t.Fatal("x") }
func panicCall() { panic("x") }
func plain()     { log.Println("x") }
func fatalValue() {
	Fatal := func(string) {}
	Fatal("x")
}
`

func TestExitsProcess(t *testing.T) {
	_, info, f := typecheck(t, "p", exitsSrc)
	want := map[string]bool{
		"exitCall": true, "fatalCall": true, "testFatal": true, "panicCall": true,
		"plain": false, "fatalValue": false,
	}
	for fn, w := range want {
		fd := funcDecl(f, fn)
		var got bool
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok && ExitsProcess(info, c) {
				got = true
			}
			return true
		})
		if got != w {
			t.Errorf("ExitsProcess in %s = %v, want %v", fn, got, w)
		}
	}
}
