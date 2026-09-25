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

// Package headerkey reports header keys that differ only in case from a
// NATS header, or are one slip away from one.
package headerkey

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

const doc = `headerkey: report header keys that miss a NATS header by case or by a slip

nats.go headers are case-preserving and lookups are exact map lookups, unlike
net/http which canonicalizes keys, and nats-server matches header names byte
for byte, so msg.Header.Get("nats-msg-id") returns "" on a message that
carries Nats-Msg-Id, and Set("nats-msg-id", v) publishes a header the server
does not recognize. The known headers are the ones nats.go defines constants
for and the ones nats-server uses.

A key that starts with Nats- but is not a NATS header is reported too when it
is within two edits of one, or is one with letters or digits appended: the
server ignores it and lookups never match. Keys are checked as arguments to
the Header methods, as index and literal keys of a Header, and where they are
compared with the key of a range over a Header.

	msg.Header.Get("nats-msg-id")            // always ""
	msg.Header.Get(jetstream.MsgIDHeader)
	msg.Header.Add("Nats-TTLSeconds", "30s") // ignored: the header is Nats-TTL

The fix replaces a miscased key with the constant from the nats.go package
the file already imports, or with the correctly cased literal. A near-miss
has no fix.`

const name = "headerkey"

var Analyzer = &analysis.Analyzer{
	Name:     name,
	Doc:      doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// headerMethods lists, per header type, the methods that take a key as
// their first argument.
var headerMethods = map[natsapi.Pkg]map[string][]string{
	natsapi.Core:  {"Header": {"Get", "Set", "Add", "Values", "Del"}},
	natsapi.Micro: {"Headers": {"Get", "Values"}},
}

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	imports := newImportIndex(pass)
	nodes := []ast.Node{(*ast.CallExpr)(nil), (*ast.IndexExpr)(nil), (*ast.CompositeLit)(nil), (*ast.BinaryExpr)(nil), (*ast.SwitchStmt)(nil)}
	ins.WithStack(nodes, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return true
		}
		for _, key := range keySites(pass, n, stack) {
			checkKey(pass, imports, key)
		}
		return true
	})
	return nil, nil
}

// keySites returns the expressions at n that name a header: the key
// argument of a header method, the index of a header, the keys of a header
// literal, and the operands compared with a header range key.
func keySites(pass *analysis.Pass, n ast.Node, stack []ast.Node) []ast.Expr {
	switch n := n.(type) {
	case *ast.CallExpr:
		if len(n.Args) > 0 && isHeaderMethod(pass, n) {
			return n.Args[:1]
		}
	case *ast.IndexExpr:
		if isHeaderType(pass.TypesInfo.TypeOf(n.X)) {
			return []ast.Expr{n.Index}
		}
	case *ast.CompositeLit:
		if !isHeaderType(pass.TypesInfo.TypeOf(n)) {
			return nil
		}
		var keys []ast.Expr
		for _, elt := range n.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				keys = append(keys, kv.Key)
			}
		}
		return keys
	case *ast.BinaryExpr:
		if n.Op != token.EQL && n.Op != token.NEQ {
			return nil
		}
		if isRangeKey(pass, stack, n.X) {
			return []ast.Expr{n.Y}
		}
		if isRangeKey(pass, stack, n.Y) {
			return []ast.Expr{n.X}
		}
	case *ast.SwitchStmt:
		if n.Tag == nil || !isRangeKey(pass, stack, n.Tag) {
			return nil
		}
		var keys []ast.Expr
		for _, s := range n.Body.List {
			keys = append(keys, s.(*ast.CaseClause).List...)
		}
		return keys
	}
	return nil
}

func checkKey(pass *analysis.Pass, imports *importIndex, key ast.Expr) {
	s, ok := natsapi.ConstString(pass.TypesInfo, key)
	if !ok {
		return
	}
	canonical, consts, ok := natsapi.Header(s)
	if !ok {
		if closest, ok := nearMiss(s); ok {
			pass.Report(analysis.Diagnostic{
				Pos:      key.Pos(),
				End:      key.End(),
				Category: name,
				Message:  fmt.Sprintf("header key %q is not a header NATS sets or reads; the closest NATS header is %q", s, closest),
			})
		}
		return
	}
	if canonical == s {
		return
	}
	pass.Report(analysis.Diagnostic{
		Pos:      key.Pos(),
		End:      key.End(),
		Category: name,
		Message:  fmt.Sprintf("header key %q does not match %q; nats.go header lookups are case-sensitive", s, canonical),
		SuggestedFixes: []analysis.SuggestedFix{{
			Message: "use the canonical header key",
			TextEdits: []analysis.TextEdit{{
				Pos:     key.Pos(),
				End:     key.End(),
				NewText: []byte(imports.replacement(key.Pos(), canonical, consts)),
			}},
		}},
	})
}

// nearMiss returns the known header an unknown Nats- key is one slip away
// from: the closest one within edit distance 2 (the lexically first on a
// tie), else the longest one the key extends with a letter or digit.
func nearMiss(key string) (string, bool) {
	lower := strings.ToLower(key)
	if !strings.HasPrefix(lower, "nats-") {
		return "", false
	}
	known := natsapi.KnownHeaders()
	best, bestDist := "", 3
	for _, h := range known {
		if d := levenshtein(lower, strings.ToLower(h)); d < bestDist {
			best, bestDist = h, d
		}
	}
	if best != "" {
		return best, true
	}
	for _, h := range known {
		lh := strings.ToLower(h)
		if len(lower) > len(lh) && strings.HasPrefix(lower, lh) && isAlnum(lower[len(lh)]) && len(h) > len(best) {
			best = h
		}
	}
	return best, best != ""
}

func isAlnum(c byte) bool {
	return 'a' <= c && c <= 'z' || 'A' <= c && c <= 'Z' || '0' <= c && c <= '9'
}

func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := range len(a) {
		cur[0] = i + 1
		for j := range len(b) {
			cost := 1
			if a[i] == b[j] {
				cost = 0
			}
			cur[j+1] = min(prev[j+1]+1, cur[j]+1, prev[j]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// isRangeKey reports whether e is the key variable of an enclosing
// for k := range h over a header, and the loop body never assigns it.
func isRangeKey(pass *analysis.Pass, stack []ast.Node, e ast.Expr) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	obj, ok := pass.TypesInfo.Uses[id].(*types.Var)
	if !ok {
		return false
	}
	for i := len(stack) - 1; i >= 0; i-- {
		rs, ok := stack[i].(*ast.RangeStmt)
		if !ok || rs.Tok != token.DEFINE {
			continue
		}
		if k, ok := rs.Key.(*ast.Ident); !ok || pass.TypesInfo.Defs[k] != obj {
			continue
		}
		return isHeaderType(pass.TypesInfo.TypeOf(rs.X)) && !assigns(pass, rs.Body, obj)
	}
	return false
}

// assigns reports whether body assigns obj, increments it or takes its
// address.
func assigns(pass *analysis.Pass, body *ast.BlockStmt, obj types.Object) bool {
	is := func(e ast.Expr) bool {
		id, ok := ast.Unparen(e).(*ast.Ident)
		return ok && pass.TypesInfo.Uses[id] == obj
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.AssignStmt:
			found = found || slices.ContainsFunc(n.Lhs, is)
		case *ast.IncDecStmt:
			found = found || is(n.X)
		case *ast.UnaryExpr:
			found = found || (n.Op == token.AND && is(n.X))
		}
		return !found
	})
	return found
}

func isHeaderMethod(pass *analysis.Pass, call *ast.CallExpr) bool {
	fn := natsapi.Callee(pass.TypesInfo, call)
	for pkg, types := range headerMethods {
		for recv, methods := range types {
			for _, m := range methods {
				if natsapi.IsMethod(fn, pkg, recv, m) {
					return true
				}
			}
		}
	}
	return false
}

func isHeaderType(t types.Type) bool {
	n, ok := t.(*types.Named)
	if !ok {
		return false
	}
	for pkg, types := range headerMethods {
		if _, ok := types[n.Obj().Name()]; ok && natsapi.IsPkg(n.Obj(), pkg) {
			return true
		}
	}
	return false
}

// importIndex records, per file, the local name under which each nats.go
// package is imported.
type importIndex struct {
	pass  *analysis.Pass
	files map[*ast.File]map[natsapi.Pkg]string
}

func newImportIndex(pass *analysis.Pass) *importIndex {
	return &importIndex{pass: pass, files: make(map[*ast.File]map[natsapi.Pkg]string)}
}

// replacement returns the source text to use for the header key at pos:
// the first constant in consts whose package the enclosing file imports,
// else the canonical literal.
func (ix *importIndex) replacement(pos token.Pos, canonical string, consts []natsapi.HeaderConst) string {
	names := ix.names(pos)
	for _, c := range consts {
		if local, ok := names[c.Pkg]; ok {
			return local + "." + c.Name
		}
	}
	return strconv.Quote(canonical)
}

func (ix *importIndex) names(pos token.Pos) map[natsapi.Pkg]string {
	var file *ast.File
	for _, f := range ix.pass.Files {
		if f.FileStart <= pos && pos < f.FileEnd {
			file = f
			break
		}
	}
	if file == nil {
		return nil
	}
	if names, ok := ix.files[file]; ok {
		return names
	}
	names := make(map[natsapi.Pkg]string)
	for _, spec := range file.Imports {
		pn := ix.pass.TypesInfo.PkgNameOf(spec)
		if pn == nil || pn.Name() == "." || pn.Name() == "_" {
			continue
		}
		for pkg := range headerPkgs {
			if natsapi.IsPkgPath(pn.Imported().Path(), pkg) {
				names[pkg] = pn.Name()
			}
		}
	}
	ix.files[file] = names
	return names
}

// headerPkgs is every package that defines header constants.
var headerPkgs = map[natsapi.Pkg]bool{natsapi.Core: true, natsapi.JetStream: true, natsapi.Micro: true}
