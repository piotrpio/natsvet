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

// schemaVersion is the version of the plan format; SKILL.md names it.
const schemaVersion = 1

// Site classes.
const (
	classMechanical = "mechanical"
	classGuided     = "guided"
	classDecision   = "decision"
	classUnmapped   = "unmapped"
)

// Plan is the JSON contract `natsvet migrate plan` writes. It holds no maps,
// so its encoding is byte-stable.
type Plan struct {
	SchemaVersion     int         `json:"schema_version"`
	Skill             string      `json:"skill"`
	Module            string      `json:"module"`
	TableNatsVersion  string      `json:"table_nats_version"`
	ModuleNatsVersion string      `json:"module_nats_version"`
	Files             []FileHash  `json:"files"`
	Counts            Counts      `json:"counts"`
	Pending           []Pending   `json:"pending_decisions"`
	StaleAnswers      []Answer    `json:"stale_answers,omitempty"`
	Steps             []Step      `json:"steps"`
	Components        []Component `json:"components"`
	Sites             []Site      `json:"sites"`
	FollowUps         []FollowUp  `json:"follow_ups"`
	Notes             []string    `json:"notes,omitempty"`
}

// FileHash is the SHA-256 of a file's content, hex-encoded.
type FileHash struct {
	File   string `json:"file"`
	SHA256 string `json:"sha256"`
}

// Counts summarizes a plan.
type Counts struct {
	LegacyUses int `json:"legacy_uses"`
	Sites      int `json:"sites"`
	Mechanical int `json:"mechanical"`
	Guided     int `json:"guided"`
	Decision   int `json:"decision"`
	Unmapped   int `json:"unmapped"`
	Skipped    int `json:"skipped"`
}

// Pending is a decision pattern still to be answered at one scope.
type Pending struct {
	Pattern string   `json:"pattern"`
	Scope   string   `json:"scope"`
	Sites   int      `json:"sites"`
	Options []string `json:"options"`
	Default string   `json:"default"`
	Reason  string   `json:"reason"`
}

// Position is a place in a module file.
type Position struct {
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
}

// Site is one unit of the migration.
type Site struct {
	ID        string         `json:"id"`
	Component string         `json:"component,omitempty"`
	Position  Position       `json:"position"`
	Symbols   []string       `json:"symbols"`
	Class     string         `json:"class"`
	Summary   string         `json:"summary"`
	Before    string         `json:"before,omitempty"`
	After     string         `json:"after,omitempty"`
	Template  string         `json:"template,omitempty"`
	Facts     []string       `json:"facts,omitempty"`
	Notes     []string       `json:"notes,omitempty"`
	Decisions []SiteDecision `json:"decisions,omitempty"`
	Ref       string         `json:"reference,omitempty"`
	Step      string         `json:"step,omitempty"`
	Skipped   bool           `json:"skipped,omitempty"`
}

// SiteDecision is one decision a site raises.
type SiteDecision struct {
	Pattern string        `json:"pattern"`
	Options []Alternative `json:"options"`
	Default string        `json:"default"`
	Reason  string        `json:"reason"`
	Choice  string        `json:"choice,omitempty"`
}

// Alternative is one answer to a decision and what it leads to.
type Alternative struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
	// After is the replacement the option leads to, with the site's other
	// decisions at their answers or defaults.
	After string `json:"after,omitempty"`
}

// Step is one change after which the module compiles. A machine step
// carries edits; any other step is done by hand (guided sites, a
// command) or waits on the sites in WaitsOn.
type Step struct {
	ID        string     `json:"id"`
	Component string     `json:"component,omitempty"`
	Kind      string     `json:"kind"`
	Summary   string     `json:"summary"`
	Machine   bool       `json:"machine"`
	Sites     []string   `json:"sites,omitempty"`
	Facts     []string   `json:"facts,omitempty"`
	Expect    []FileHash `json:"expect,omitempty"`
	Edits     []Edit     `json:"edits,omitempty"`
	WaitsOn   []string   `json:"waits_on,omitempty"`
	Command   string     `json:"command,omitempty"`
}

// Edit replaces bytes [Start, End) of File with New, in the coordinates
// of the file after all earlier steps.
type Edit struct {
	File  string `json:"file"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	New   string `json:"new"`
}

// Component is a set of legacy handles and the sites acting on them.
type Component struct {
	ID        string   `json:"id"`
	Handles   []string `json:"handles"`
	Blocked   []Block  `json:"blocked,omitempty"`
	Skipped   bool     `json:"skipped,omitempty"`
	OneCommit bool     `json:"one_commit,omitempty"`
	Steps     []string `json:"steps"`
	Sites     []string `json:"sites"`
}

// Block is a reason a component keeps its legacy handle.
type Block struct {
	Position Position `json:"position"`
	Reason   string   `json:"reason"`
}

// FollowUp is an improvement the migration makes possible.
type FollowUp struct {
	Site      string    `json:"site"`
	Kind      string    `json:"kind"`
	Summary   string    `json:"summary"`
	Candidate *Position `json:"candidate,omitempty"`
}

// Answer records a decision in natsvet-migrate.json.
type Answer struct {
	Pattern string `json:"pattern"`
	Scope   string `json:"scope"`
	Choice  string `json:"choice"`
}

// Decisions is the content of natsvet-migrate.json.
type Decisions struct {
	Version int      `json:"version"`
	Answers []Answer `json:"answers"`
}
