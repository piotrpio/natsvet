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

// Package tablegen derives the natsapi tables from the nats.go version pinned
// by the testdata module: the NATS header constants and the legacy JetStream
// API symbols. The tables are committed; a test regenerates them and fails
// on any difference so a nats.go bump cannot leave them stale.
package tablegen

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/constant"
	"go/format"
	"go/token"
	"go/types"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

const natsModule = "github.com/nats-io/nats.go"

// File names of the generated tables inside internal/natsapi.
const (
	HeadersFile   = "headers_table.go"
	LegacyFile    = "legacy_table.go"
	DurationsFile = "durations_table.go"
)

// headerPkgs lists the packages scanned for header constants in the order
// that decides fix preference: a header defined in several packages is
// rewritten to the constant of the first imported one.
var headerPkgs = []struct{ path, name, ident string }{
	{natsModule + "/jetstream", "jetstream", "JetStream"},
	{natsModule, "nats", "Core"},
	{natsModule + "/micro", "micro", "Micro"},
}

// legacyFiles are the files of package nats that hold the legacy JetStream,
// KeyValue and ObjectStore API.
var legacyFiles = []string{"js.go", "jsm.go", "jserrors.go", "kv.go", "object.go"}

// HeaderConst names a constant that defines a NATS header.
type HeaderConst struct {
	Pkg  string // package name: jetstream, nats or micro
	Name string
}

// Tables is what Collect extracts from nats.go.
type Tables struct {
	// Headers maps a header value to the constants defining it, in fix
	// preference order.
	Headers map[string][]HeaderConst
	// Legacy holds "Type", "Func" and "Type.Method" keys of the legacy API.
	Legacy map[string]bool
	// Durations holds "pkg.Type" for every exported type declared as
	// time.Duration; go/types only keeps the int64 underlying type.
	Durations map[string]bool
	// Version is the nats.go module version the tables were derived from.
	Version string
}

// Generated holds the rendered Go source files.
type Generated struct {
	Headers   []byte
	Legacy    []byte
	Durations []byte
}

// Available reports whether the pinned nats.go can be loaded from dir's
// module without network access.
func Available(dir string) error {
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", natsModule)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go list -m %s: %v: %s", natsModule, err, bytes.TrimSpace(out))
	}
	if len(bytes.TrimSpace(out)) == 0 {
		return fmt.Errorf("%s is not in the module cache", natsModule)
	}
	return nil
}

// Collect loads the nats.go packages through dir's module and extracts the
// tables.
func Collect(dir string) (*Tables, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedModule,
		Dir: dir,
	}
	patterns := make([]string, len(headerPkgs))
	for i, p := range headerPkgs {
		patterns[i] = p.path
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return nil, err
	}
	if packages.PrintErrors(pkgs) > 0 {
		return nil, fmt.Errorf("loading %s failed", natsModule)
	}
	byPath := make(map[string]*packages.Package, len(pkgs))
	for _, p := range pkgs {
		byPath[p.PkgPath] = p
	}
	t := &Tables{Headers: make(map[string][]HeaderConst), Legacy: make(map[string]bool), Durations: make(map[string]bool)}
	for _, hp := range headerPkgs {
		p, ok := byPath[hp.path]
		if !ok {
			return nil, fmt.Errorf("package %s not loaded", hp.path)
		}
		if p.Module != nil {
			t.Version = p.Module.Version
		}
		collectHeaders(t, p, hp.name)
		collectDurations(t, p, hp.name)
	}
	collectLegacy(t, byPath[natsModule])
	return t, nil
}

func collectHeaders(t *Tables, p *packages.Package, pkgName string) {
	scope := p.Types.Scope()
	for _, name := range scope.Names() {
		c, ok := scope.Lookup(name).(*types.Const)
		if !ok || !c.Exported() || c.Val().Kind() != constant.String {
			continue
		}
		v := constant.StringVal(c.Val())
		if strings.HasPrefix(v, "Nats-") {
			t.Headers[v] = append(t.Headers[v], HeaderConst{pkgName, name})
		}
	}
}

func collectLegacy(t *Tables, p *packages.Package) {
	for i, f := range p.Syntax {
		if !slices.Contains(legacyFiles, filepath.Base(p.CompiledGoFiles[i])) {
			continue
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.GenDecl:
				if d.Tok != token.TYPE {
					continue
				}
				for _, s := range d.Specs {
					ts := s.(*ast.TypeSpec)
					if !ts.Name.IsExported() {
						continue
					}
					t.Legacy[ts.Name.Name] = true
					if it, ok := ts.Type.(*ast.InterfaceType); ok {
						collectInterfaceMethods(t, ts.Name.Name, it)
					}
				}
			case *ast.FuncDecl:
				if !d.Name.IsExported() {
					continue
				}
				if d.Recv == nil {
					t.Legacy[d.Name.Name] = true
					continue
				}
				if r := receiverIdent(d.Recv.List[0].Type); r != "" && ast.IsExported(r) {
					t.Legacy[r+"."+d.Name.Name] = true
				}
			}
		}
	}
}

// collectDurations records exported types declared as time.Duration.
func collectDurations(t *Tables, p *packages.Package, pkgName string) {
	for _, f := range p.Syntax {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, s := range gd.Specs {
				ts := s.(*ast.TypeSpec)
				sel, ok := ts.Type.(*ast.SelectorExpr)
				if !ok || !ts.Name.IsExported() {
					continue
				}
				if x, ok := sel.X.(*ast.Ident); ok && x.Name == "time" && sel.Sel.Name == "Duration" {
					t.Durations[pkgName+"."+ts.Name.Name] = true
				}
			}
		}
	}
}

// collectInterfaceMethods records the methods an interface declares itself;
// methods of embedded interfaces are recorded under the embedded interface,
// which is also the receiver go/types reports for them.
func collectInterfaceMethods(t *Tables, iface string, it *ast.InterfaceType) {
	for _, m := range it.Methods.List {
		for _, name := range m.Names {
			if name.IsExported() {
				t.Legacy[iface+"."+name.Name] = true
			}
		}
	}
}

// receiverIdent returns the type name of a method receiver expression,
// looking through pointers and type parameters.
func receiverIdent(e ast.Expr) string {
	switch e := e.(type) {
	case *ast.StarExpr:
		return receiverIdent(e.X)
	case *ast.IndexExpr:
		return receiverIdent(e.X)
	case *ast.IndexListExpr:
		return receiverIdent(e.X)
	case *ast.Ident:
		return e.Name
	}
	return ""
}

// Generate renders the tables as natsapi source files.
func (t *Tables) Generate() (*Generated, error) {
	h, err := t.renderHeaders()
	if err != nil {
		return nil, err
	}
	l, err := t.renderLegacy()
	if err != nil {
		return nil, err
	}
	d, err := t.renderDurations()
	if err != nil {
		return nil, err
	}
	return &Generated{Headers: h, Legacy: l, Durations: d}, nil
}

const licenseHeader = `// Copyright 2026 Synadia Communications Inc.
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

`

func (t *Tables) preamble(b *bytes.Buffer) {
	b.WriteString(licenseHeader)
	fmt.Fprintf(b, "// Code generated by tablegen from %s %s; DO NOT EDIT.\n\n", natsModule, t.Version)
	b.WriteString("package natsapi\n\n")
}

func (t *Tables) renderHeaders() ([]byte, error) {
	var b bytes.Buffer
	t.preamble(&b)
	b.WriteString("// headers maps every NATS header defined by nats.go to the constants that\n")
	b.WriteString("// define it, in fix preference order.\n")
	b.WriteString("var headers = map[string][]HeaderConst{\n")
	keys := make([]string, 0, len(t.Headers))
	for k := range t.Headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "\t%q: {", k)
		for i, c := range t.Headers[k] {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "{%s, %q}", pkgIdent(c.Pkg), c.Name)
		}
		b.WriteString("},\n")
	}
	b.WriteString("}\n")
	return format.Source(b.Bytes())
}

func pkgIdent(name string) string {
	for _, p := range headerPkgs {
		if p.name == name {
			return p.ident
		}
	}
	panic("unknown package " + name)
}

func (t *Tables) renderLegacy() ([]byte, error) {
	var b bytes.Buffer
	t.preamble(&b)
	b.WriteString("// legacySymbols holds the exported types, functions and methods of the\n")
	b.WriteString("// legacy JetStream API in package nats, as \"Type\", \"Func\" and\n")
	b.WriteString("// \"Type.Method\".\n")
	b.WriteString("var legacySymbols = map[string]bool{\n")
	keys := make([]string, 0, len(t.Legacy))
	for k := range t.Legacy {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "\t%q: true,\n", k)
	}
	b.WriteString("}\n")
	return format.Source(b.Bytes())
}

func (t *Tables) renderDurations() ([]byte, error) {
	var b bytes.Buffer
	t.preamble(&b)
	b.WriteString("// durationTypes holds every exported nats.go type declared as time.Duration,\n")
	b.WriteString("// keyed by package name and type name.\n")
	b.WriteString("var durationTypes = map[string]bool{\n")
	keys := make([]string, 0, len(t.Durations))
	for k := range t.Durations {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "\t%q: true,\n", k)
	}
	b.WriteString("}\n")
	return format.Source(b.Bytes())
}
