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
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

const usesDeclSrc = `package p

func WithPublishAsyncErrHandler() {}
func WithPublishAsyncAckHandler() {}

type JetStream interface {
	PublishAsyncComplete()
	PublishAsyncPending() int
}

type JetStreamContext interface {
	JetStream
}
`

const usesUseSrc = `package p

func use(js JetStreamContext) {
	WithPublishAsyncErrHandler()
	js.PublishAsyncComplete()
}
`

func TestPackageUses(t *testing.T) {
	info := typecheckFiles(t, string(JetStream), usesDeclSrc, usesUseSrc)
	tests := []struct {
		recv, name string
		want       bool
	}{
		{"", "WithPublishAsyncErrHandler", true},
		{"JetStream", "PublishAsyncComplete", true},
		{"", "WithPublishAsyncAckHandler", false},
		{"JetStream", "PublishAsyncPending", false},
		{"JetStreamContext", "PublishAsyncComplete", false},
	}
	for _, tt := range tests {
		if got := PackageUses(info, JetStream, tt.recv, tt.name); got != tt.want {
			t.Errorf("PackageUses(%q, %q) = %v, want %v", tt.recv, tt.name, got, tt.want)
		}
	}
	other := typecheckFiles(t, "example.com/other", usesDeclSrc, usesUseSrc)
	if PackageUses(other, JetStream, "", "WithPublishAsyncErrHandler") {
		t.Error("PackageUses matched a same-named function from another package")
	}
	declOnly := typecheckFiles(t, string(JetStream), usesDeclSrc)
	if PackageUses(declOnly, JetStream, "", "WithPublishAsyncErrHandler") {
		t.Error("PackageUses counted a declaration as a use")
	}
}

func typecheckFiles(t *testing.T, path string, srcs ...string) *types.Info {
	t.Helper()
	fset := token.NewFileSet()
	var files []*ast.File
	for i, src := range srcs {
		f, err := parser.ParseFile(fset, "src"+string(rune('a'+i))+".go", src, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	info := &types.Info{
		Uses: make(map[*ast.Ident]types.Object),
		Defs: make(map[*ast.Ident]types.Object),
	}
	conf := types.Config{Importer: importer.Default()}
	if _, err := conf.Check(path, fset, files, info); err != nil {
		t.Fatal(err)
	}
	return info
}
