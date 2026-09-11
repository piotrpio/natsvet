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
// header nats.go defines.
package headerkey

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

const doc = `headerkey: report header keys that differ only in case from a NATS header

nats.go headers are case-preserving and lookups are exact map lookups, unlike
net/http which canonicalizes keys, so msg.Header.Get("nats-msg-id") returns ""
on a message that carries Nats-Msg-Id, and Set("nats-msg-id", v) publishes a
header the server does not recognize.

	msg.Header.Get("nats-msg-id")      // always ""
	msg.Header.Get(jetstream.MsgIDHeader)

The fix replaces the key with the constant from the nats.go package the file
already imports, or with the correctly cased literal.`

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
	ins.Preorder([]ast.Node{(*ast.CallExpr)(nil), (*ast.IndexExpr)(nil)}, func(n ast.Node) {
		var key ast.Expr
		switch n := n.(type) {
		case *ast.CallExpr:
			if len(n.Args) == 0 || !isHeaderMethod(pass, n) {
				return
			}
			key = n.Args[0]
		case *ast.IndexExpr:
			if !isHeaderType(pass.TypesInfo.TypeOf(n.X)) {
				return
			}
			key = n.Index
		}
		s, ok := natsapi.ConstString(pass.TypesInfo, key)
		if !ok {
			return
		}
		canonical, consts, ok := natsapi.Header(s)
		if !ok || canonical == s {
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
	})
	return nil, nil
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
