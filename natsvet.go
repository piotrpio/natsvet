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

// Package natsvet is a suite of go/analysis analyzers for programs that use
// github.com/nats-io/nats.go. Analyzers returns the rules that run by default;
// OptIn returns the rules that must be enabled explicitly with their
// -<rule>.enable flag.
package natsvet

import (
	"golang.org/x/tools/go/analysis"

	"github.com/piotrpio/natsvet/analyzers/consumerconfig"
	"github.com/piotrpio/natsvet/analyzers/ctxdeadline"
	"github.com/piotrpio/natsvet/analyzers/duration"
	"github.com/piotrpio/natsvet/analyzers/headerkey"
	"github.com/piotrpio/natsvet/analyzers/kvconfig"
	"github.com/piotrpio/natsvet/analyzers/legacyjs"
	"github.com/piotrpio/natsvet/analyzers/streamconfig"
	"github.com/piotrpio/natsvet/analyzers/subject"
	"github.com/piotrpio/natsvet/analyzers/syncsub"
)

// Analyzers returns the default-on rules.
func Analyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		consumerconfig.Analyzer,
		ctxdeadline.Analyzer,
		duration.Analyzer,
		headerkey.Analyzer,
		kvconfig.Analyzer,
		streamconfig.Analyzer,
		subject.Analyzer,
		syncsub.Analyzer,
	}
}

// OptIn returns the rules that report nothing unless enabled with their
// enable flag.
func OptIn() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		legacyjs.Analyzer,
	}
}

// All returns every rule, default-on first.
func All() []*analysis.Analyzer {
	return append(Analyzers(), OptIn()...)
}
