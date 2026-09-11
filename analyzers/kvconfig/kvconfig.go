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

// Package kvconfig reports KeyValue and ObjectStore bucket names, history
// limits and keys that nats.go rejects client-side.
package kvconfig

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

const doc = `kvconfig: report KV bucket names, history limits and keys nats.go rejects

The checks are the ones in nats.go's jetstream/kv.go and jetstream/object.go
(bucketValid, keyValid, searchKeyValid, KeyValueMaxHistory) and fire only on
constants, so a report is a guaranteed ErrInvalidBucketName,
ErrInvalidStoreName, ErrHistoryTooLarge or ErrInvalidKey at runtime.

	js.KeyValue(ctx, "my.bucket")   // ErrInvalidBucketName
	kv.Put(ctx, "user name", data)  // ErrInvalidKey
	kv.Watch(ctx, "users.*")        // fine: filters may use wildcards`

const name = "kvconfig"

var Analyzer = &analysis.Analyzer{
	Name:     name,
	Doc:      doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

const maxHistory = 64

var configTypes = []natsapi.TypeRef{
	{Pkg: natsapi.JetStream, Name: "KeyValueConfig"},
	{Pkg: natsapi.Core, Name: "KeyValueConfig"},
	{Pkg: natsapi.JetStream, Name: "ObjectStoreConfig"},
	{Pkg: natsapi.Core, Name: "ObjectStoreConfig"},
}

// argKind says how a method's argument is validated.
type argKind int

const (
	bucketArg argKind = iota
	keyArg
	searchArg      // a single filter
	searchSliceArg // a []string literal of filters
	searchVariadic // filters from arg onward
)

type hook struct {
	pkg  natsapi.Pkg
	recv string
	name string
	arg  int
	kind argKind
}

var hooks = func() []hook {
	var hs []hook
	add := func(pkg natsapi.Pkg, recv string, arg int, kind argKind, names ...string) {
		for _, n := range names {
			hs = append(hs, hook{pkg, recv, n, arg, kind})
		}
	}
	for _, p := range []struct {
		pkg natsapi.Pkg
		arg int
	}{{natsapi.JetStream, 1}, {natsapi.Core, 0}} {
		add(p.pkg, "KeyValueManager", p.arg, bucketArg, "KeyValue", "DeleteKeyValue")
		add(p.pkg, "ObjectStoreManager", p.arg, bucketArg, "ObjectStore", "DeleteObjectStore")
		add(p.pkg, "KeyValue", p.arg, keyArg, "Get", "GetRevision", "Put", "PutString", "Create", "Update", "Delete", "Purge")
		add(p.pkg, "KeyValue", p.arg, searchArg, "Watch", "History")
		add(p.pkg, "KeyValue", p.arg, searchSliceArg, "WatchFiltered")
		add(p.pkg, "KeyValue", p.arg, searchVariadic, "ListKeysFiltered")
	}
	return hs
}()

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	report := func(n ast.Node, msg string) {
		pass.Report(analysis.Diagnostic{Pos: n.Pos(), End: n.End(), Category: name, Message: msg})
	}
	assigned := make(map[*ast.BlockStmt]map[string]bool)
	ins.WithStack([]ast.Node{(*ast.CompositeLit)(nil), (*ast.CallExpr)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return false
		}
		switch n := n.(type) {
		case *ast.CompositeLit:
			body := natsapi.EnclosingFuncBody(stack)
			if _, seen := assigned[body]; !seen {
				assigned[body] = natsapi.AssignedFields(body)
			}
			checkConfig(pass, n, assigned[body], report)
		case *ast.CallExpr:
			checkCall(pass, n, report)
		}
		return true
	})
	return nil, nil
}

func checkConfig(pass *analysis.Pass, lit *ast.CompositeLit, assigned map[string]bool, report func(ast.Node, string)) {
	fields, _, ok := natsapi.CompositeFields(pass.TypesInfo, lit, configTypes...)
	if !ok {
		return
	}
	c := natsapi.NewFields(pass.TypesInfo, fields, false, nil, assigned)
	if _, present := c.Expr("Bucket"); present {
		if b, ok := c.Str("Bucket"); ok && !natsapi.BucketValid(b) {
			report(lit, bucketMsg(b))
		}
	}
	if h, ok := c.Int("History"); ok && h > maxHistory {
		report(lit, fmt.Sprintf("KV history %d exceeds the maximum of %d", h, maxHistory))
	}
}

func checkCall(pass *analysis.Pass, call *ast.CallExpr, report func(ast.Node, string)) {
	fn := natsapi.Callee(pass.TypesInfo, call)
	if fn == nil {
		return
	}
	for _, h := range hooks {
		if !natsapi.IsMethod(fn, h.pkg, h.recv, h.name) || len(call.Args) <= h.arg {
			continue
		}
		arg := call.Args[h.arg]
		switch h.kind {
		case bucketArg:
			if b, ok := natsapi.ConstString(pass.TypesInfo, arg); ok && !natsapi.BucketValid(b) {
				report(arg, bucketMsg(b))
			}
		case keyArg:
			if k, ok := natsapi.ConstString(pass.TypesInfo, arg); ok && !natsapi.KeyValid(k) {
				report(arg, keyMsg(k))
			}
		case searchArg:
			checkSearch(pass, arg, report)
		case searchSliceArg:
			if lit, ok := ast.Unparen(arg).(*ast.CompositeLit); ok {
				for _, elt := range lit.Elts {
					checkSearch(pass, elt, report)
				}
			}
		case searchVariadic:
			if call.Ellipsis.IsValid() {
				return
			}
			for _, a := range call.Args[h.arg:] {
				checkSearch(pass, a, report)
			}
		}
		return
	}
}

func checkSearch(pass *analysis.Pass, arg ast.Expr, report func(ast.Node, string)) {
	if k, ok := natsapi.ConstString(pass.TypesInfo, arg); ok && !natsapi.SearchKeyValid(k) {
		report(arg, fmt.Sprintf("invalid KV key filter %q: filters may only contain [-/_=.a-zA-Z0-9*] and a trailing >", k))
	}
}

func bucketMsg(b string) string {
	return fmt.Sprintf("invalid bucket name %q: bucket names may only contain [a-zA-Z0-9_-]", b)
}

func keyMsg(k string) string {
	return fmt.Sprintf("invalid KV key %q: keys may only contain [-/_=.a-zA-Z0-9]", k)
}
