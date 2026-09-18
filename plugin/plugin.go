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

// Package plugin registers natsvet as a golangci-lint module plugin named
// natsvet. A .custom-gcl.yml that names this module with this package as
// its import path builds a golangci-lint binary in which
// linters.settings.custom.natsvet with type module enables every default-on
// rule; the settings block turns opt-in rules on and default-on rules off.
package plugin

import (
	"fmt"
	"slices"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"

	"github.com/piotrpio/natsvet"
)

func init() {
	register.Plugin("natsvet", New)
}

// settings is the linters.settings.custom.natsvet.settings block.
type settings struct {
	// Enable names opt-in rules to run.
	Enable []string `json:"enable"`
	// Disable names default-on rules not to run.
	Disable []string `json:"disable"`
}

type plugin struct {
	analyzers []*analysis.Analyzer
}

// New builds the plugin from the settings block golangci-lint decoded from
// its configuration. Unknown keys and unknown rule names are errors, so a
// typo fails the run instead of silently running the default set. Every
// opt-in analyzer's enable flag is set on each call, listed or not, because
// analyzers are package-level values shared between constructions.
func New(raw any) (register.LinterPlugin, error) {
	s, err := register.DecodeSettings[settings](raw)
	if err != nil {
		return nil, fmt.Errorf("natsvet: %w", err)
	}
	known := map[string]bool{}
	for _, a := range natsvet.All() {
		known[a.Name] = true
	}
	for _, list := range []struct {
		key   string
		names []string
	}{{"enable", s.Enable}, {"disable", s.Disable}} {
		for _, n := range list.names {
			if !known[n] {
				return nil, fmt.Errorf("natsvet: unknown rule %q in %s", n, list.key)
			}
		}
	}
	var out []*analysis.Analyzer
	for _, a := range natsvet.Analyzers() {
		if !slices.Contains(s.Disable, a.Name) {
			out = append(out, a)
		}
	}
	for _, a := range natsvet.OptIn() {
		on := slices.Contains(s.Enable, a.Name)
		if err := a.Flags.Set("enable", fmt.Sprint(on)); err != nil {
			return nil, fmt.Errorf("natsvet: %s: %w", a.Name, err)
		}
		if on {
			out = append(out, a)
		}
	}
	return &plugin{analyzers: out}, nil
}

func (p *plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return p.analyzers, nil
}

// GetLoadMode asks for type information: every rule resolves nats.go symbols
// through go/types.
func (p *plugin) GetLoadMode() string {
	return register.LoadModeTypesInfo
}
