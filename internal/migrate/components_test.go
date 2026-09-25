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
	"strings"
	"testing"
)

func componentsOf(t *testing.T, patterns ...string) (*graph, []*component) {
	t.Helper()
	prog := loadTestdata(t, patterns...)
	sites := inventory(prog)
	g := buildGraph(prog, sites)
	return g, g.components(sites)
}

func TestComponentSpansPackages(t *testing.T) {
	_, comps := componentsOf(t, "./migrate/app", "./migrate/worker")
	var service *component
	for _, c := range comps {
		for _, h := range c.handles {
			if h.ident.Name == "JS" && h.kind == handleField {
				service = c
			}
		}
	}
	if service == nil {
		t.Fatal("no component holds the Service.JS field")
	}
	files := map[string]bool{}
	for _, s := range service.sites {
		files[s.file.rel] = true
	}
	if !files["migrate/app/app.go"] || !files["migrate/worker/worker.go"] {
		t.Errorf("component sites span %v, want app.go and worker.go", files)
	}
	for _, h := range service.handles {
		if h.ident.Name == "JS" && (h.sibling != "JSNew" || !h.threadable) {
			t.Errorf("Service.JS: sibling %q threadable %v (%s)", h.sibling, h.threadable, h.why)
		}
	}
}

func TestIndependentHandles(t *testing.T) {
	_, comps := componentsOf(t, "./migrate/independent")
	if len(comps) != 2 {
		t.Fatalf("got %d components, want 2", len(comps))
	}
	for _, c := range comps {
		if len(c.boundaries) != 0 {
			t.Errorf("component %s has boundaries %v", c.id, c.boundaries)
		}
	}
}

func TestExternalBoundary(t *testing.T) {
	_, comps := componentsOf(t, "./migrate/boundary")
	if len(comps) != 1 {
		t.Fatalf("got %d components, want 1", len(comps))
	}
	b := comps[0].boundaries
	if len(b) != 1 || !strings.Contains(b[0].reason, "outside the loaded packages") {
		t.Errorf("boundaries = %v, want one call outside the loaded packages", b)
	}
}
