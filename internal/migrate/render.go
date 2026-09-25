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
			if st.Component == "" {
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
	"join": strings.Join,
}

var mdTemplate = template.Must(template.New("plan").Funcs(mdFuncs).Parse(`# Migration plan for {{.Module}}

{{.Skill}}. Plan schema version {{.SchemaVersion}}; mapping verified against nats.go {{.TableNatsVersion}}; the module requires {{or .ModuleNatsVersion "an unknown nats.go"}}.
{{range .Notes}}
> {{.}}
{{end}}
## Summary

- Legacy uses: {{.Counts.LegacyUses}} in {{.Counts.Sites}} sites
- Mechanical: {{.Counts.Mechanical}}, guided: {{.Counts.Guided}}, decision: {{.Counts.Decision}}, unmapped: {{.Counts.Unmapped}}, skipped: {{.Counts.Skipped}}
- Components: {{len .Components}}, steps: {{len .Steps}}
{{if .Pending}}
## Pending decisions

Ask the user each question, record the answer in natsvet-migrate.json, and plan again.
{{range .Pending}}
- **{{.Pattern}}** ({{.Scope}}, {{.Sites}} {{if eq .Pattern "component"}}component{{else}}site{{end}}{{if ne .Sites 1}}s{{end}}): {{.Reason}}. Options: {{join .Options ", "}}. Default: **{{.Default}}**.
{{- end}}
{{end}}{{if .StaleAnswers}}
## Stale answers

These answers in natsvet-migrate.json name a scope that no longer exists:
{{range .StaleAnswers}}
- {{.Pattern}} = {{.Choice}} at {{.Scope}}
{{- end}}
{{end}}{{$plan := .}}{{range .Steps}}{{if eq .Kind "go-get"}}
## Step {{.ID}}: raise nats.go

{{.Summary}}.

    {{.Command}}
{{end}}{{end}}{{range .Components}}
## Component {{.ID}}{{if .Skipped}} (skipped){{end}}

Handles: {{join .Handles ", "}}.{{if .OneCommit}} Safe to apply in one commit.{{end}}
{{range .Blocked}}
- Blocked at {{.Position.File}}:{{.Position.Line}}:{{.Position.Column}}: {{.Reason}}; removal and rename are omitted.
{{- end}}
{{range steps $plan .Steps}}
### Step {{.ID}} ({{.Kind}}{{if .Machine}}, machine edits{{end}})

{{.Summary}}.
{{- if .WaitsOn}} Waits on: {{join .WaitsOn ", "}}.{{end}}
{{range .Facts}}
- {{.}}
{{- end}}
{{range sites $plan .Sites}}
#### Site {{.ID}}: {{.Class}}

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
{{end}}{{end}}{{end}}{{with loose .}}
## Sites outside components
{{range .}}
### Step {{.ID}} ({{.Kind}}{{if .Machine}}, machine edits{{end}})

{{.Summary}}.
{{range .Facts}}
- {{.}}
{{- end}}
{{range sites $plan .Sites}}
#### Site {{.ID}}: {{.Class}}

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
{{- end}}
{{end}}{{end}}{{end}}{{if .FollowUps}}
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
