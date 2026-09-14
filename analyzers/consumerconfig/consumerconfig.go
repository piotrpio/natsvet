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

// Package consumerconfig reports consumer configurations that nats-server
// rejects at create or update time.
package consumerconfig

import (
	"fmt"
	"go/ast"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

const doc = `consumerconfig: report consumer configurations the server rejects

Every check mirrors one in nats-server's checkConsumerCfg and fires only
when the involved fields are constants in the same composite literal, so a
report is a guaranteed runtime error, reported at the literal instead of as
a JetStreamError from CreateConsumer.

	jetstream.ConsumerConfig{
		FilterSubject:  "orders.new",
		FilterSubjects: []string{"orders.paid"}, // both set: rejected
	}`

const name = "consumerconfig"

var Analyzer = &analysis.Analyzer{
	Name:     name,
	Doc:      doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

var (
	jsConfig      = natsapi.TypeRef{Pkg: natsapi.JetStream, Name: "ConsumerConfig"}
	jsOrdered     = natsapi.TypeRef{Pkg: natsapi.JetStream, Name: "OrderedConsumerConfig"}
	legacyConfig  = natsapi.TypeRef{Pkg: natsapi.Core, Name: "ConsumerConfig"}
	legacyAliases = map[string]string{"IdleHeartbeat": "Heartbeat"}
	validGroup    = regexp.MustCompile(`^[a-zA-Z0-9/_=-]{1,16}$`)
)

const maxDescription = 4096

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	masker := natsapi.NewMasker(pass.TypesInfo)
	ins.WithStack([]ast.Node{(*ast.CompositeLit)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return false
		}
		lit := n.(*ast.CompositeLit)
		fields, matched, ok := natsapi.CompositeFields(pass.TypesInfo, lit, jsConfig, jsOrdered, legacyConfig)
		if !ok {
			return true
		}
		c := &cfg{natsapi.NewFields(pass.TypesInfo, fields, matched == legacyConfig, legacyAliases, masker.Masked(stack))}
		report := func(msg string) {
			pass.Report(analysis.Diagnostic{
				Pos:      lit.Pos(),
				End:      lit.End(),
				Category: name,
				Message:  "consumer config: " + msg,
			})
		}
		for _, check := range checks {
			check(c, report)
		}
		return true
	})
	return nil, nil
}

// cfg is the constant-only view of one literal plus the push/pull mode
// derived from DeliverSubject.
type cfg struct {
	*natsapi.Fields
}

// mode reports whether the literal is a push consumer (constant non-empty
// DeliverSubject) or a pull consumer (absent or empty), when known.
func (c *cfg) mode() (push, pull bool) {
	ds, ok := c.Str("DeliverSubject")
	return ok && ds != "", ok && ds == ""
}

var checks = []func(c *cfg, report func(string)){
	checkNames,
	checkNegatives,
	checkAckFlowControl,
	checkBackOff,
	checkDescription,
	checkPush,
	checkPull,
	checkFilters,
	checkDeliverPolicy,
	checkSampling,
	checkFlowControlHeartbeat,
	checkDurableName,
	checkPriority,
}

func invalidAssetName(s string) bool {
	return strings.ContainsAny(s, " \t\r\n\f.*>\\/")
}

func checkNames(c *cfg, report func(string)) {
	if v, ok := c.Str("Name"); ok && v != "" && invalidAssetName(v) {
		report(`consumer name can not contain '.', '*', '>', '\', '/' or whitespace`)
	}
	if v, ok := c.Str("Durable"); ok && v != "" && invalidAssetName(v) {
		report(`consumer durable name can not contain '.', '*', '>', '\', '/' or whitespace`)
	}
}

func checkNegatives(c *cfg, report func(string)) {
	if v, ok := c.Int("Replicas"); ok && v < 0 {
		report("replicas count cannot be negative")
	}
	if vals := c.Durs("BackOff"); len(vals) > 0 {
		for _, d := range vals {
			if d < 0 {
				report("consumer backoff needs to be positive")
				break
			}
		}
	}
	if v, ok := c.Dur("AckWait"); ok && v < 0 {
		report("consumer ack wait needs to be positive")
	}
}

func checkAckFlowControl(c *cfg, report func(string)) {
	if p, ok := c.Enum("AckPolicy", "AckExplicitPolicy"); !ok || p != "AckFlowControlPolicy" {
		return
	}
	if _, pull := c.mode(); pull {
		report("flow control ack policy requires a push based consumer")
	}
	if fc, ok := c.Bool("FlowControl"); ok && !fc {
		report("flow control ack policy requires flow control")
	}
	if hb, ok := c.Dur("IdleHeartbeat"); ok && hb != time.Second {
		report("flow control ack policy heartbeat needs to be 1s")
	}
	if v, ok := c.Int("MaxAckPending"); ok && v <= 0 {
		report("flow control ack policy requires max ack pending")
	}
	aw, awOK := c.Dur("AckWait")
	n, bkOK := c.SliceLen("BackOff")
	if awOK && aw != 0 || bkOK && n > 0 {
		report("flow control ack policy requires unset ack wait")
	}
	if v, ok := c.Int("MaxDeliver"); ok && v > 0 {
		report("flow control ack policy requires unset max deliver")
	}
}

func checkBackOff(c *cfg, report func(string)) {
	n, ok := c.SliceLen("BackOff")
	if !ok || n == 0 {
		return
	}
	if md, ok := c.Int("MaxDeliver"); ok && md > 0 && n > int(md) {
		report("max deliver is required to be > length of backoff values")
	}
}

func checkDescription(c *cfg, report func(string)) {
	if v, ok := c.Str("Description"); ok && len(v) > maxDescription {
		report(fmt.Sprintf("consumer description is too long, maximum allowed is %d", maxDescription))
	}
}

func checkPush(c *cfg, report func(string)) {
	push, _ := c.mode()
	if !push {
		return
	}
	ds, _ := c.Str("DeliverSubject")
	if !natsapi.SubjectIsLiteral(ds) {
		report("consumer deliver subject has wildcards")
	}
	if !natsapi.IsValidSubject(ds) {
		report("invalid push consumer deliver subject")
	}
	if v, ok := c.Int("MaxWaiting"); ok && v != 0 {
		report("consumer in push mode can not set max waiting")
	}
	if v, ok := c.Int("MaxAckPending"); ok && v > 0 {
		if p, ok := c.Enum("AckPolicy", "AckExplicitPolicy"); ok && p == "AckNonePolicy" {
			report("consumer requires ack policy for max ack pending")
		}
	}
	if hb, ok := c.Dur("IdleHeartbeat"); ok && hb > 0 && hb < 100*time.Millisecond {
		report("consumer idle heartbeat needs to be >= 100ms")
	}
}

func checkPull(c *cfg, report func(string)) {
	_, pull := c.mode()
	if !pull {
		return
	}
	if v, ok := c.Int("RateLimit"); ok && v > 0 {
		report("consumer in pull mode can not have rate limit set")
	}
	if v, ok := c.Int("MaxWaiting"); ok && v < 0 {
		report("consumer max waiting needs to be positive")
	}
	if hb, ok := c.Dur("IdleHeartbeat"); ok && hb > 0 {
		report("consumer idle heartbeat requires a push based consumer")
	}
	if fc, ok := c.Bool("FlowControl"); ok && fc {
		report("consumer flow control requires a push based consumer")
	}
	if v, ok := c.Int("MaxRequestBatch"); ok && v < 0 {
		report("consumer max request batch needs to be > 0")
	}
	if v, ok := c.Dur("MaxRequestExpires"); ok && v != 0 && v < time.Millisecond {
		report("consumer max request expires needs to be >= 1ms")
	}
}

func checkFilters(c *cfg, report func(string)) {
	single, singleOK := c.Str("FilterSubject")
	multi, complete, _ := c.Strs("FilterSubjects")
	if n, ok := c.SliceLen("FilterSubjects"); singleOK && single != "" && ok && n > 0 {
		report("consumer cannot have both FilterSubject and FilterSubjects specified")
	}
	if singleOK && single != "" && !natsapi.IsValidSubject(single) {
		report(fmt.Sprintf("invalid filter subject %q", single))
	}
	for _, f := range multi {
		if f == "" {
			report("consumer filter in FilterSubjects cannot be empty")
			break
		}
	}
	for _, f := range multi {
		if f != "" && !natsapi.IsValidSubject(f) {
			report(fmt.Sprintf("invalid filter subject %q", f))
		}
	}
	if !complete {
		return
	}
	filters := multi
	if singleOK && single != "" {
		filters = append([]string{single}, multi...)
	}
	for i, a := range filters {
		for j, b := range filters {
			if i != j && a != "" && b != "" && natsapi.SubjectIsSubsetMatch(a, b) {
				report("consumer subject filters cannot overlap")
				return
			}
		}
	}
}

func checkDeliverPolicy(c *cfg, report func(string)) {
	policy, ok := c.Enum("DeliverPolicy", "DeliverAllPolicy")
	if !ok {
		return
	}
	seq, seqOK := c.Int("OptStartSeq")
	tm := c.Ptr("OptStartTime")
	badStart := func(dp, start string) {
		report(fmt.Sprintf("consumer delivery policy is deliver %s, but optional start %s is also set", dp, start))
	}
	notSet := func(dp, what string) {
		report(fmt.Sprintf("consumer delivery policy is deliver %s, but optional %s is not set", dp, what))
	}
	names := map[string]string{
		"DeliverAllPolicy":            "all",
		"DeliverLastPolicy":           "last",
		"DeliverNewPolicy":            "new",
		"DeliverLastPerSubjectPolicy": "last per subject",
	}
	switch policy {
	case "DeliverAllPolicy", "DeliverLastPolicy", "DeliverNewPolicy", "DeliverLastPerSubjectPolicy":
		if seqOK && seq > 0 {
			badStart(names[policy], "sequence")
		}
		if tm == natsapi.PtrSet {
			badStart(names[policy], "time")
		}
		if policy == "DeliverLastPerSubjectPolicy" {
			single, singleOK := c.Str("FilterSubject")
			n, nOK := c.SliceLen("FilterSubjects")
			if singleOK && single == "" && nOK && n == 0 {
				notSet("last per subject", "filter subject")
			}
		}
	case "DeliverByStartSequencePolicy":
		if seqOK && seq == 0 {
			notSet("by start sequence", "start sequence")
		}
		if tm == natsapi.PtrSet {
			badStart("by start sequence", "time")
		}
	case "DeliverByStartTimePolicy":
		if tm == natsapi.PtrNil {
			notSet("by start time", "start time")
		}
		if seqOK && seq != 0 {
			badStart("by start time", "start sequence")
		}
	}
}

func checkSampling(c *cfg, report func(string)) {
	v, ok := c.Str("SampleFrequency")
	if !ok || v == "" {
		return
	}
	n, err := strconv.Atoi(strings.TrimSuffix(v, "%"))
	if err != nil || n < 0 {
		report("failed to parse consumer sampling configuration")
	}
}

func checkFlowControlHeartbeat(c *cfg, report func(string)) {
	fc, ok := c.Bool("FlowControl")
	if !ok || !fc {
		return
	}
	if hb, ok := c.Dur("IdleHeartbeat"); ok && hb == 0 {
		report("consumer with flow control also needs heartbeats")
	}
}

func checkDurableName(c *cfg, report func(string)) {
	d, dOK := c.Str("Durable")
	n, nOK := c.Str("Name")
	if dOK && nOK && d != "" && n != "" && d != n {
		report("Consumer Durable and Name have to be equal if both are provided")
	}
}

func checkPriority(c *cfg, report func(string)) {
	policy, ok := c.Enum("PriorityPolicy", "PriorityPolicyNone")
	if !ok {
		return
	}
	groups, _, _ := c.Strs("PriorityGroups")
	nGroups, groupsOK := c.SliceLen("PriorityGroups")
	if policy != "PriorityPolicyNone" {
		if push, _ := c.mode(); push {
			report("priority groups can not be used with push consumers")
		}
		if groupsOK && nGroups == 0 {
			report("Setting PriorityPolicy requires at least one PriorityGroup to be set")
		}
		for _, g := range groups {
			if g == "" {
				report("Group name cannot be an empty string")
			} else if !validGroup.MatchString(g) {
				report("Valid priority group name must match A-Z, a-z, 0-9, -_/=)+ and may not exceed 16 characters")
			}
		}
		return
	}
	if groupsOK && nGroups > 0 {
		report("consumer can not have priority groups when policy is none")
	}
	if v, ok := c.Dur("PinnedTTL"); ok && v > 0 {
		report("PinnedTTL cannot be set when PriorityPolicy is none")
	}
}
