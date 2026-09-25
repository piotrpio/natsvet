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

package migrate

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

const natsModule = "github.com/nats-io/nats.go"

// program is the loaded module: every file the plan covers, each once,
// with the type information of the package variant it was taken from.
type program struct {
	fset        *token.FileSet
	files       []*srcFile
	moduleDir   string
	modulePath  string
	natsVersion string
	// byPath indexes files by absolute path.
	byPath map[string]*srcFile
	// js is the jetstream package of the module's nats.go, nil when that
	// nats.go has none.
	js *types.Package
}

// srcFile is one Go file of the loaded packages.
type srcFile struct {
	path string // absolute
	rel  string // relative to the module root, slash-separated
	src  []byte
	ast  *ast.File
	pkg  *packages.Package
	test bool
}

func (f *srcFile) info() *types.Info { return f.pkg.TypesInfo }

// load type-checks the packages matching patterns in dir as one program.
// Non-test files come from the plain package variants, so that a
// declaration used from another package is the same object everywhere;
// test files come from the test variants. Objects are compared by
// declaration position (see objKey), which also joins the two variants.
func load(dir string, patterns []string, tests bool) (*program, error) {
	cfg := &packages.Config{
		Mode:  packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps | packages.NeedModule,
		Dir:   dir,
		Tests: tests,
		Fset:  token.NewFileSet(),
	}
	roots, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}
	jsCfg := *cfg
	jsCfg.Mode, jsCfg.Tests = packages.NeedName|packages.NeedTypes, false
	var jsPkg *types.Package
	if js, err := packages.Load(&jsCfg, natsModule+"/jetstream"); err == nil && len(js) == 1 && len(js[0].Errors) == 0 {
		jsPkg = js[0].Types
	}
	var errs []string
	natsVersion := ""
	packages.Visit(roots, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			errs = append(errs, e.Error())
		}
		if p.PkgPath == natsModule && p.Module != nil {
			natsVersion = p.Module.Version
			if p.Module.Replace != nil && p.Module.Replace.Version != "" {
				natsVersion = p.Module.Replace.Version
			}
		}
	})
	if len(errs) > 0 {
		slices.Sort(errs)
		return nil, errors.New("packages do not load or type-check:\n\t" + strings.Join(slices.Compact(errs), "\n\t"))
	}
	prog := &program{fset: cfg.Fset, natsVersion: natsVersion, byPath: make(map[string]*srcFile), js: jsPkg}
	for _, p := range roots {
		if p.Module != nil && p.Module.Main && prog.moduleDir == "" {
			prog.moduleDir, prog.modulePath = p.Module.Dir, p.Module.Path
		}
	}
	if prog.moduleDir == "" {
		return nil, fmt.Errorf("no main module found for %v", patterns)
	}
	// Plain variants first, so their files win; then test variants, which
	// contribute only their _test.go files.
	slices.SortFunc(roots, func(a, b *packages.Package) int {
		if av, bv := isTestVariant(a), isTestVariant(b); av != bv {
			if av {
				return 1
			}
			return -1
		}
		return strings.Compare(a.ID, b.ID)
	})
	for _, p := range roots {
		if strings.HasSuffix(p.ID, ".test") || p.Module == nil || !p.Module.Main {
			continue
		}
		for i, path := range p.CompiledGoFiles {
			if i >= len(p.Syntax) {
				break
			}
			test := strings.HasSuffix(path, "_test.go")
			if _, seen := prog.byPath[path]; seen || (isTestVariant(p) && !test) {
				continue
			}
			src, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			rel, err := filepath.Rel(prog.moduleDir, path)
			if err != nil {
				return nil, err
			}
			f := &srcFile{path: path, rel: filepath.ToSlash(rel), src: src, ast: p.Syntax[i], pkg: p, test: test}
			prog.byPath[path] = f
			prog.files = append(prog.files, f)
		}
	}
	slices.SortFunc(prog.files, func(a, b *srcFile) int { return strings.Compare(a.rel, b.rel) })
	return prog, nil
}

func isTestVariant(p *packages.Package) bool {
	return strings.Contains(p.ID, " [") || strings.HasSuffix(p.PkgPath, "_test")
}

// objKey identifies an object across package variants: its declaring
// file, offset and name. Objects without a position (universe, unsafe)
// key by name alone.
type objKey string

func (prog *program) key(obj types.Object) objKey {
	if obj == nil {
		return ""
	}
	if !obj.Pos().IsValid() {
		return objKey(obj.Name())
	}
	p := prog.fset.Position(obj.Pos())
	return objKey(fmt.Sprintf("%s:%d:%s", p.Filename, p.Offset, obj.Name()))
}

// inLoaded reports whether obj is declared in a file of the loaded set.
func (prog *program) inLoaded(obj types.Object) bool {
	if obj == nil || !obj.Pos().IsValid() {
		return false
	}
	_, ok := prog.byPath[prog.fset.Position(obj.Pos()).Filename]
	return ok
}

// fileOf returns the loaded file containing pos.
func (prog *program) fileOf(pos token.Pos) *srcFile {
	return prog.byPath[prog.fset.Position(pos).Filename]
}

// offset returns the byte offset of pos in its file.
func (prog *program) offset(pos token.Pos) int {
	return prog.fset.Position(pos).Offset
}
