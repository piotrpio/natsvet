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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"
)

// decisionsFile is the default name of the answers file at the module root.
const decisionsFile = "natsvet-migrate.json"

// decisionsVersion is the version of natsvet-migrate.json this planner
// reads.
const decisionsVersion = 2

// Decision patterns.
const (
	patSubscribeTarget = "subscribe-target"
	patAck             = "ack"
	patPushOnly        = "push-only-option"
	patChanMaxAck      = "channel-max-ack-pending"
	patSharedHandler   = "shared-handler"
	patComponent       = "component"
)

// patternDef is a decision pattern's fixed option set.
type patternDef struct {
	options []Alternative
}

var patterns = map[string]patternDef{
	patSubscribeTarget: {options: []Alternative{
		{ID: "pull", Summary: "a pull consumer: Consume for callbacks, Messages for sync and channel forms"},
		{ID: "push", Summary: "a push consumer (Subscribe and QueueSubscribe only): CreateOrUpdatePushConsumer, or PushConsumer when bound"},
		{ID: "defer", Summary: "keep the legacy subscription for now; the component keeps its legacy handle"},
	}},
	patAck: {options: []Alternative{
		{ID: "after-handler", Summary: "ack after the handler returns, as the legacy wrapper did"},
		{ID: "explicit", Summary: "ack explicitly on each handler path"},
		{ID: "none", Summary: "AckNonePolicy: no acks at all"},
	}},
	patPushOnly: {options: []Alternative{
		{ID: "drop", Summary: "drop the push-only option; pull consumers have no equivalent"},
		{ID: "push", Summary: "use a push consumer for this subscription instead"},
	}},
	patChanMaxAck: {options: []Alternative{
		{ID: "keep", Summary: "keep MaxAckPending at the channel capacity, as legacy set it"},
		{ID: "server-default", Summary: "leave MaxAckPending to the server default"},
	}},
	patSharedHandler: {options: []Alternative{
		{ID: "split", Summary: "split the handler: a jetstream.Msg copy for the JetStream subscription, the original for core"},
		{ID: "adapter", Summary: "keep one handler and adapt jetstream.Msg to *nats.Msg at the JetStream subscription"},
	}},
	patComponent: {options: []Alternative{
		{ID: "migrate", Summary: "migrate the component"},
		{ID: "skip", Summary: "keep the component on the legacy API; its sites appear in no step"},
	}},
}

// decision is one decision a site or component raises.
type decision struct {
	pattern string
	def     string
	reason  string
	choice  string
	scope   string // the scope of the answer that chose it
	// after is the replacement text each option leads to, by option id.
	after map[string]string
}

func (d *decision) effective() string {
	if d.choice != "" {
		return d.choice
	}
	return d.def
}

// answerSet holds the answers of a decisions file and which of them were
// used.
type answerSet struct {
	answers []Answer
	used    []bool
}

// readDecisions reads the answers file: path when given, else
// natsvet-migrate.json at the module root when it exists.
func readDecisions(moduleDir, path string) (*answerSet, error) {
	explicit := path != ""
	if !explicit {
		path = moduleDir + string(os.PathSeparator) + decisionsFile
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !explicit && errors.Is(err, fs.ErrNotExist) {
			return &answerSet{}, nil
		}
		return nil, err
	}
	var d Decisions
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&d); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if d.Version != decisionsVersion {
		return nil, fmt.Errorf("%s: version %d; this natsvet reads version %d, whose ids name code rather than positions (component:<pkg>.<Func>#<handle> or component:<pkg>.<Type>.<field>, site:<pkg>.<Func>#<Symbol>@<hash>): plan again and answer under the new plan's ids", path, d.Version, decisionsVersion)
	}
	for _, a := range d.Answers {
		def, ok := patterns[a.Pattern]
		if !ok {
			return nil, fmt.Errorf("%s: unknown pattern %q", path, a.Pattern)
		}
		if !slices.ContainsFunc(def.options, func(o Alternative) bool { return o.ID == a.Choice }) {
			var ids []string
			for _, o := range def.options {
				ids = append(ids, o.ID)
			}
			return nil, fmt.Errorf("%s: pattern %q has no option %q (options: %s)", path, a.Pattern, a.Choice, strings.Join(ids, ", "))
		}
		switch {
		case a.Scope == "module":
		case strings.HasPrefix(a.Scope, "component:") && len(a.Scope) > len("component:"):
		case strings.HasPrefix(a.Scope, "site:") && len(a.Scope) > len("site:"):
			if a.Pattern == patComponent {
				return nil, fmt.Errorf("%s: pattern %q is answered per component or module, not per site", path, a.Pattern)
			}
		default:
			return nil, fmt.Errorf("%s: pattern %q: unknown scope %q (want module, component:<id> or site:<id>)", path, a.Pattern, a.Scope)
		}
	}
	return &answerSet{answers: d.Answers, used: make([]bool, len(d.Answers))}, nil
}

// lookup returns the narrowest answer to pattern for a site of a
// component: site, then component, then module.
func (as *answerSet) lookup(pattern, siteID, compID string) (choice, scope string) {
	best, rank := -1, 0
	for i, a := range as.answers {
		if a.Pattern != pattern {
			continue
		}
		r := 0
		switch {
		case a.Scope == "module":
			r = 1
		case compID != "" && a.Scope == "component:"+compID:
			r = 2
		case siteID != "" && a.Scope == "site:"+siteID:
			r = 3
		}
		if r > rank {
			best, rank = i, r
		}
	}
	if best < 0 {
		return "", ""
	}
	as.used[best] = true
	return as.answers[best].Choice, as.answers[best].Scope
}

// stale returns the answers whose site or component scope names nothing
// in the plan.
func (as *answerSet) stale(sites, comps map[string]bool) []Answer {
	var out []Answer
	for _, a := range as.answers {
		switch {
		case strings.HasPrefix(a.Scope, "site:") && !sites[strings.TrimPrefix(a.Scope, "site:")]:
			out = append(out, a)
		case strings.HasPrefix(a.Scope, "component:") && !comps[strings.TrimPrefix(a.Scope, "component:")]:
			out = append(out, a)
		}
	}
	return out
}
