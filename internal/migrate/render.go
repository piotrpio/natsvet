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
	"bytes"
	"encoding/json"
	"io"
	"slices"
	"strings"
	"text/template"
)

// writeJSON writes the plan as indented JSON.
func writeJSON(w io.Writer, plan *Plan) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(plan); err != nil {
		return err
	}
	_, err := w.Write(buf.Bytes())
	return err
}

var mdFuncs = template.FuncMap{
	"code": func(s string) string {
		fence := "```"
		for strings.Contains(s, fence) {
			fence += "`"
		}
		return fence + "go\n" + s + "\n" + fence
	},
	"sites": func(plan *Plan, ids []string) []Site {
		byID := make(map[string]Site)
		for _, s := range plan.Sites {
			byID[s.ID] = s
		}
		var out []Site
		for _, id := range ids {
			if s, ok := byID[id]; ok {
				out = append(out, s)
			}
		}
		return out
	},
	"loose": func(plan *Plan) []Step {
		var out []Step
		for _, st := range plan.Steps {
			if st.Component == "" && st.Kind != stepGoGet {
				out = append(out, st)
			}
		}
		return out
	},
	"steps": func(plan *Plan, ids []string) []Step {
		var out []Step
		for _, st := range plan.Steps {
			for _, id := range ids {
				if st.ID == id {
					out = append(out, st)
				}
			}
		}
		return out
	},
	// own returns the sites a step shows: those listed by it that belong
	// to it, so that a site several steps list is shown once.
	"own": func(plan *Plan, st Step) []Site {
		var out []Site
		for _, s := range plan.Sites {
			if s.Step == st.ID && slices.Contains(st.Sites, s.ID) {
				out = append(out, s)
			}
		}
		return out
	},
	// stepless returns the sites of a class that no step shows: unmapped
	// ones, or (with class "") skipped ones.
	"stepless": func(plan *Plan, class string) []Site {
		var out []Site
		for _, s := range plan.Sites {
			if s.Step == "" && ((class == "" && s.Class != classUnmapped) || s.Class == class) {
				out = append(out, s)
			}
		}
		return out
	},
	"component": func(plan *Plan, id string) Component {
		for _, c := range plan.Components {
			if c.ID == id {
				return c
			}
		}
		return Component{ID: id}
	},
	"files": func(plan *Plan, c Component) []string {
		var out []string
		for _, s := range plan.Sites {
			if slices.Contains(c.Sites, s.ID) && !slices.Contains(out, s.Position.File) {
				out = append(out, s.Position.File)
			}
		}
		return out
	},
	"join": strings.Join,
}

var mdTemplate = template.Must(template.New("plan").Funcs(mdFuncs).Parse(`{{define "site"}}
#### Site {{.ID}}: {{.Class}}{{if .Function}} in {{.Function}}{{end}}

{{.Summary}}.{{if .Ref}} See {{.Ref}}.{{end}}
{{if .Before}}
Before:

{{code .Before}}
{{end}}{{if .After}}
After:

{{code .After}}
{{end}}{{if .Template}}
Template:

{{code .Template}}
{{end}}{{range .Facts}}
- Fact: {{.}}
{{- end}}{{range .Notes}}
- Note: {{.}}
{{- end}}{{range .Decisions}}
- Decision **{{.Pattern}}**{{if .Choice}} answered **{{.Choice}}**{{else}} pending, default **{{.Default}}**{{end}}: {{.Reason}}.
{{- range .Options}}
  - {{.ID}}: {{.Summary}}
{{- end}}
{{- end}}
{{end}}{{define "step"}}
### Step {{.ID}} ({{.Kind}}{{if .Machine}}, machine edits{{end}})

{{.Summary}}.
{{- if .WaitsOn}} Waits on: {{join .WaitsOn ", "}}.{{end}}
{{range .Facts}}
- {{.}}
{{- end}}{{if .Before}}
Before:

{{code .Before}}

After:

{{code .After}}
{{end}}{{end}}# Migration plan for {{.Module}}

{{.Skill}}. Plan schema version {{.SchemaVersion}}; mapping verified against nats.go {{.TableNatsVersion}}; the module requires {{or .ModuleNatsVersion "an unknown nats.go"}}.
{{range .Notes}}
> {{.}}
{{end}}
## Summary

- Legacy uses: {{.Counts.LegacyUses}} in {{.Counts.Sites}} sites
- Mechanical: {{.Counts.Mechanical}}, guided: {{.Counts.Guided}}, decision: {{.Counts.Decision}}, unmapped: {{.Counts.Unmapped}}, skipped: {{.Counts.Skipped}}
- Components: {{len .Components}}, steps: {{len .Steps}}
{{$plan := .}}{{if .Pending}}
## Pending decisions

Ask the user each question, record the answer in natsvet-migrate.json, and plan again.
{{range .Pending}}
- **{{.Pattern}}** ({{.Scope}}, {{if .Components}}{{len .Components}} component{{if ne (len .Components) 1}}s{{end}}{{else}}{{.Sites}} site{{if ne .Sites 1}}s{{end}}{{end}}): {{.Reason}}. Options: {{join .Options ", "}}. Default: **{{.Default}}**.
{{- range .Components}}{{with component $plan .}}
  - {{.ID}}: handles {{join .Handles ", "}}; files {{join (files $plan .) ", "}}{{if .Functions}}; functions {{join .Functions ", "}}{{end}}{{if .TestOnly}}; test code only{{end}}
{{- end}}{{end}}
{{- end}}
{{end}}{{if .StaleAnswers}}
## Stale answers

These answers in natsvet-migrate.json name a scope that no longer exists:
{{range .StaleAnswers}}
- {{.Pattern}} = {{.Choice}} at {{.Scope}}
{{- end}}
{{end}}{{range .Steps}}{{if eq .Kind "go-get"}}
## Step {{.ID}}: raise nats.go

{{.Summary}}.

    {{.Command}}
{{end}}{{end}}{{range .Components}}
## Component {{.ID}}{{if .Skipped}} (skipped){{end}}

Handles: {{join .Handles ", "}}.{{if .OneCommit}} Safe to apply in one commit.{{end}}{{if .TestOnly}} Test code only.{{end}}
{{range .Blocked}}
- Blocked at {{.Position.File}}:{{.Position.Line}}:{{.Position.Column}}: {{.Reason}}; the finish step is omitted.
{{- end}}
{{range steps $plan .Steps}}{{template "step" .}}{{range own $plan .}}{{template "site" .}}{{end}}{{end}}{{end}}{{with loose .}}
## Sites outside components
{{range .}}{{template "step" .}}{{range own $plan .}}{{template "site" .}}{{end}}{{end}}{{end}}{{with stepless . "unmapped"}}
## Unmapped sites

No jetstream counterpart; the code stays on the legacy API until the user decides otherwise.
{{range .}}{{template "site" .}}{{end}}{{end}}{{with stepless . ""}}
## Skipped sites
{{range .}}{{template "site" .}}{{end}}{{end}}{{if .FollowUps}}
## Follow-ups

Not needed to finish the migration:
{{range .FollowUps}}
- {{.Site}} ({{.Kind}}): {{.Summary}}
{{- end}}
{{end}}`))

// writeMarkdown renders the plan as a guide.
func writeMarkdown(w io.Writer, plan *Plan) error {
	return mdTemplate.Execute(w, plan)
}
