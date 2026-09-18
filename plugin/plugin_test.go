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

package plugin

import (
	"slices"
	"strings"
	"testing"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/piotrpio/natsvet"
)

func TestRegistered(t *testing.T) {
	if _, err := register.GetPlugin("natsvet"); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMode(t *testing.T) {
	p, err := New(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.GetLoadMode(); got != register.LoadModeTypesInfo {
		t.Errorf("GetLoadMode() = %q, want %q", got, register.LoadModeTypesInfo)
	}
}

func TestSettings(t *testing.T) {
	defaultOn := names(natsvet.Analyzers())
	tests := []struct {
		name     string
		settings any
		want     []string // analyzer names, in order; nil with wantErr
		enabled  []string // opt-in analyzers whose enable flag must be true
		wantErr  string
	}{
		{name: "nil", settings: nil, want: defaultOn},
		{name: "empty map", settings: map[string]any{}, want: defaultOn},
		{name: "enable opt-in", settings: map[string]any{"enable": []any{"legacyjs"}}, want: append(slices.Clone(defaultOn), "legacyjs"), enabled: []string{"legacyjs"}},
		{name: "disable default-on", settings: map[string]any{"disable": []any{"drain"}}, want: without(defaultOn, "drain")},
		{name: "both", settings: map[string]any{"enable": []any{"legacyjs"}, "disable": []any{"drain", "msgloop"}}, want: append(without(defaultOn, "drain", "msgloop"), "legacyjs"), enabled: []string{"legacyjs"}},
		{name: "default-on in enable", settings: map[string]any{"enable": []any{"drain"}}, want: defaultOn},
		{name: "opt-in in disable", settings: map[string]any{"disable": []any{"legacyjs"}}, want: defaultOn},
		{name: "unknown in enable", settings: map[string]any{"enable": []any{"legacyj"}}, wantErr: "legacyj"},
		{name: "unknown in disable", settings: map[string]any{"disable": []any{"drian"}}, wantErr: "drian"},
		{name: "unknown key", settings: map[string]any{"enabled": []any{"legacyjs"}}, wantErr: "enabled"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := New(tt.settings)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("New() error = %v, want one naming %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			as, err := p.BuildAnalyzers()
			if err != nil {
				t.Fatal(err)
			}
			if got := names(as); !slices.Equal(got, tt.want) {
				t.Errorf("analyzers = %v, want %v", got, tt.want)
			}
			for _, a := range natsvet.OptIn() {
				want := slices.Contains(tt.enabled, a.Name)
				if got := a.Flags.Lookup("enable").Value.String() == "true"; got != want {
					t.Errorf("%s enable flag = %v, want %v", a.Name, got, want)
				}
			}
		})
	}
}

func names(as []*analysis.Analyzer) []string {
	out := make([]string, len(as))
	for i, a := range as {
		out[i] = a.Name
	}
	return out
}

func without(list []string, drop ...string) []string {
	var out []string
	for _, s := range list {
		if !slices.Contains(drop, s) {
			out = append(out, s)
		}
	}
	return out
}
