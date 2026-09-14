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
	"go/token"
	"go/types"
)

// Masker decides which fields of a config literal must be treated as
// unknown because the value may change between the literal and the point
// where it leaves the enclosing function's hands.
//
// A literal that is a direct call argument or return value is a temporary:
// nothing is masked. A literal bound to a variable is scanned forward,
// statement by statement, until the variable is handed off — passed to a
// call by value, returned, sent on a channel, or passed by pointer to a
// nats.go method — and only fields assigned before that point are masked.
// If a pointer to the variable reaches any other call, the variable is a
// method receiver, or it is captured by a closure before the hand-off, or
// no hand-off is found in the block, every field assigned anywhere in the
// function is masked instead.
type Masker struct {
	info  *types.Info
	whole map[*ast.BlockStmt]map[string]bool
}

// NewMasker returns a Masker for one pass.
func NewMasker(info *types.Info) *Masker {
	return &Masker{info: info, whole: make(map[*ast.BlockStmt]map[string]bool)}
}

// Masked returns the fields to treat as unknown for the composite literal
// at the top of stack (as produced by inspector.WithStack).
func (m *Masker) Masked(stack []ast.Node) map[string]bool {
	body := EnclosingFuncBody(stack)
	if body == nil {
		return nil
	}
	// Climb from the literal through enclosing &, parentheses and outer
	// composite literals to the value that is bound or passed.
	i := len(stack) - 1
	for i > 0 {
		switch p := stack[i-1].(type) {
		case *ast.UnaryExpr:
			if p.Op != token.AND {
				return m.wholeFunc(body)
			}
		case *ast.ParenExpr, *ast.KeyValueExpr, *ast.CompositeLit:
		default:
			goto climbed
		}
		i--
	}
climbed:
	value, isExpr := stack[i].(ast.Expr)
	if i == 0 || !isExpr {
		return m.wholeFunc(body)
	}
	var obj types.Object
	var stmtIdx int
	switch p := stack[i-1].(type) {
	case *ast.CallExpr:
		for _, a := range p.Args {
			if a == value {
				return nil
			}
		}
		return m.wholeFunc(body)
	case *ast.ReturnStmt:
		return nil
	case *ast.AssignStmt:
		obj = boundObject(m.info, p.Lhs, p.Rhs, value)
		stmtIdx = i - 1
	case *ast.ValueSpec:
		obj = boundSpecObject(m.info, p, value)
		// The enclosing statement is the DeclStmt two levels up.
		stmtIdx = i - 3
	default:
		return m.wholeFunc(body)
	}
	if obj == nil || stmtIdx < 1 {
		return m.wholeFunc(body)
	}
	stmt, ok := stack[stmtIdx].(ast.Stmt)
	if !ok {
		return m.wholeFunc(body)
	}
	list := blockList(stack[stmtIdx-1])
	if list == nil {
		return m.wholeFunc(body)
	}
	start := -1
	for j, s := range list {
		if s == stmt {
			start = j
			break
		}
	}
	if start < 0 {
		return m.wholeFunc(body)
	}
	masked := make(map[string]bool)
	for _, s := range list[start+1:] {
		for f := range assignedIn(s) {
			masked[f] = true
		}
		switch m.classify(s, obj) {
		case escape:
			return m.wholeFunc(body)
		case handoff:
			return masked
		}
	}
	return m.wholeFunc(body)
}

func (m *Masker) wholeFunc(body *ast.BlockStmt) map[string]bool {
	if w, ok := m.whole[body]; ok {
		return w
	}
	w := AssignedFields(body)
	m.whole[body] = w
	return w
}

// boundObject returns the object of the identifier that value is assigned
// to in a := or = statement, or nil when value is not bound to a plain
// identifier.
func boundObject(info *types.Info, lhs, rhs []ast.Expr, value ast.Expr) types.Object {
	if len(lhs) != len(rhs) {
		return nil
	}
	for i, r := range rhs {
		if r != value {
			continue
		}
		if id, ok := lhs[i].(*ast.Ident); ok && id.Name != "_" {
			return info.ObjectOf(id)
		}
	}
	return nil
}

func boundSpecObject(info *types.Info, spec *ast.ValueSpec, value ast.Expr) types.Object {
	if len(spec.Names) != len(spec.Values) {
		return nil
	}
	for i, v := range spec.Values {
		if v == value && spec.Names[i].Name != "_" {
			return info.ObjectOf(spec.Names[i])
		}
	}
	return nil
}

// blockList returns the statement list of a block-like node.
func blockList(n ast.Node) []ast.Stmt {
	switch n := n.(type) {
	case *ast.BlockStmt:
		return n.List
	case *ast.CaseClause:
		return n.Body
	case *ast.CommClause:
		return n.Body
	}
	return nil
}

type use int

const (
	none use = iota
	handoff
	escape
)

// classify reports whether statement s hands the variable off, lets it
// escape, or neither. A statement that does both escapes.
func (m *Masker) classify(s ast.Stmt, obj types.Object) use {
	result := none
	isVar := func(e ast.Expr) bool {
		id, ok := ast.Unparen(e).(*ast.Ident)
		return ok && m.info.ObjectOf(id) == obj
	}
	isAddr := func(e ast.Expr) bool {
		u, ok := ast.Unparen(e).(*ast.UnaryExpr)
		return ok && u.Op == token.AND && isVar(u.X)
	}
	_, isPtr := obj.Type().(*types.Pointer)
	natsCall := func(c *ast.CallExpr) bool {
		fn := Callee(m.info, c)
		return fn != nil && (IsPkg(fn, Core) || IsPkg(fn, JetStream))
	}
	note := func(u use) {
		if u > result {
			result = u
		}
	}
	ast.Inspect(s, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.FuncLit:
			ast.Inspect(n.Body, func(inner ast.Node) bool {
				if e, ok := inner.(ast.Expr); ok && isVar(e) {
					note(escape)
				}
				return true
			})
			return false
		case *ast.CallExpr:
			if sel, ok := ast.Unparen(n.Fun).(*ast.SelectorExpr); ok && isVar(sel.X) {
				note(escape)
			}
			for _, a := range n.Args {
				switch {
				case isAddr(a), isVar(a) && isPtr:
					if natsCall(n) {
						note(handoff)
					} else {
						note(escape)
					}
				case isVar(a):
					note(handoff)
				}
			}
		case *ast.UnaryExpr:
			if n.Op == token.AND && isVar(n.X) && !addrIsCallArg(s, n) {
				note(escape)
			}
		case *ast.ReturnStmt:
			for _, r := range n.Results {
				if isVar(r) {
					note(handoff)
				}
			}
		case *ast.SendStmt:
			if isVar(n.Value) {
				note(handoff)
			}
		}
		return true
	})
	return result
}

// addrIsCallArg reports whether the &x expression u is directly an
// argument of a call inside s (those are classified by the call).
func addrIsCallArg(s ast.Stmt, u *ast.UnaryExpr) bool {
	found := false
	ast.Inspect(s, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			for _, a := range c.Args {
				if ast.Unparen(a) == u {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

// assignedIn returns the selector names assigned in a single statement,
// including nested blocks.
func assignedIn(s ast.Stmt) map[string]bool {
	return AssignedFields(&ast.BlockStmt{List: []ast.Stmt{s}})
}
