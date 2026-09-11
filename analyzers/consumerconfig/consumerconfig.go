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
	"go/types"
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
	enumPkgs      = []natsapi.Pkg{natsapi.JetStream, natsapi.Core}
	validGroup    = regexp.MustCompile(`^[a-zA-Z0-9/_=-]{1,16}$`)
)

const maxDescription = 4096

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	ins.Preorder([]ast.Node{(*ast.CompositeLit)(nil)}, func(n ast.Node) {
		lit := n.(*ast.CompositeLit)
		fields, matched, ok := natsapi.CompositeFields(pass.TypesInfo, lit, jsConfig, jsOrdered, legacyConfig)
		if !ok {
			return
		}
		c := &cfg{info: pass.TypesInfo, fields: fields, legacy: matched == legacyConfig}
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
	})
	return nil, nil
}

// cfg is a constant-only view of one literal. Every accessor returns the
// value and whether it is known: an absent field is the zero value and
// known; a non-constant field is unknown.
type cfg struct {
	info   *types.Info
	fields map[string]ast.Expr
	legacy bool
}

func (c *cfg) expr(field string) (ast.Expr, bool) {
	if c.legacy {
		if alias, ok := legacyAliases[field]; ok {
			field = alias
		}
	}
	e, ok := c.fields[field]
	return e, ok
}

func (c *cfg) str(field string) (string, bool) {
	e, present := c.expr(field)
	if !present {
		return "", true
	}
	return natsapi.ConstString(c.info, e)
}

func (c *cfg) num(field string) (int64, bool) {
	e, present := c.expr(field)
	if !present {
		return 0, true
	}
	return natsapi.ConstInt(c.info, e)
}

func (c *cfg) dur(field string) (time.Duration, bool) {
	e, present := c.expr(field)
	if !present {
		return 0, true
	}
	return natsapi.ConstDuration(c.info, e)
}

func (c *cfg) boolean(field string) (bool, bool) {
	e, present := c.expr(field)
	if !present {
		return false, true
	}
	return natsapi.ConstBool(c.info, e)
}

// strs returns the constant elements of a slice field, whether every
// element was constant, and whether the field is a literal (or absent) at
// all.
func (c *cfg) strs(field string) (vals []string, complete, known bool) {
	e, present := c.expr(field)
	if !present {
		return nil, true, true
	}
	vals, complete = natsapi.SliceConstStrings(c.info, e)
	if !complete && vals == nil {
		if _, isLit := ast.Unparen(e).(*ast.CompositeLit); !isLit {
			return nil, false, false
		}
	}
	return vals, complete, true
}

// sliceLen returns the element count of a slice field literal; an absent
// or nil field has length 0; a non-literal value is unknown.
func (c *cfg) sliceLen(field string) (int, bool) {
	e, present := c.expr(field)
	if !present {
		return 0, true
	}
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return 0, e.Name == "nil"
	case *ast.CompositeLit:
		return len(e.Elts), true
	}
	return 0, false
}

// durs returns the constant elements of a []time.Duration field and the
// literal's length; known is false for a non-literal value.
func (c *cfg) durs(field string) (vals []time.Duration, n int, known bool) {
	e, present := c.expr(field)
	if !present {
		return nil, 0, true
	}
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return nil, 0, e.Name == "nil"
	case *ast.CompositeLit:
		for _, elt := range e.Elts {
			if d, ok := natsapi.ConstDuration(c.info, elt); ok {
				vals = append(vals, d)
			}
		}
		return vals, len(e.Elts), true
	}
	return nil, 0, false
}

// enum returns the name of the enum constant a field holds; zero is the
// name of the type's zero value, used for an absent field or a literal 0.
func (c *cfg) enum(field, zero string) (string, bool) {
	e, present := c.expr(field)
	if !present {
		return zero, true
	}
	if v, ok := natsapi.ConstEnum(c.info, e, enumPkgs...); ok {
		return v, true
	}
	if v, ok := natsapi.ConstInt(c.info, e); ok && v == 0 {
		return zero, true
	}
	return "", false
}

type ptrState int

const (
	ptrNil ptrState = iota
	ptrSet
	ptrUnknown
)

func (c *cfg) ptr(field string) ptrState {
	e, present := c.expr(field)
	if !present {
		return ptrNil
	}
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		if e.Name == "nil" {
			return ptrNil
		}
	case *ast.UnaryExpr:
		return ptrSet
	}
	return ptrUnknown
}

// mode reports whether the literal is a push consumer (constant non-empty
// DeliverSubject) or a pull consumer (absent or empty), when known.
func (c *cfg) mode() (push, pull bool) {
	ds, ok := c.str("DeliverSubject")
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
	if v, ok := c.str("Name"); ok && v != "" && invalidAssetName(v) {
		report(`consumer name can not contain '.', '*', '>', '\', '/' or whitespace`)
	}
	if v, ok := c.str("Durable"); ok && v != "" && invalidAssetName(v) {
		report(`consumer durable name can not contain '.', '*', '>', '\', '/' or whitespace`)
	}
}

func checkNegatives(c *cfg, report func(string)) {
	if v, ok := c.num("Replicas"); ok && v < 0 {
		report("replicas count cannot be negative")
	}
	if vals, _, _ := c.durs("BackOff"); len(vals) > 0 {
		for _, d := range vals {
			if d < 0 {
				report("consumer backoff needs to be positive")
				break
			}
		}
	}
	if v, ok := c.dur("AckWait"); ok && v < 0 {
		report("consumer ack wait needs to be positive")
	}
}

func checkAckFlowControl(c *cfg, report func(string)) {
	if p, ok := c.enum("AckPolicy", "AckExplicitPolicy"); !ok || p != "AckFlowControlPolicy" {
		return
	}
	if _, pull := c.mode(); pull {
		report("flow control ack policy requires a push based consumer")
	}
	if fc, ok := c.boolean("FlowControl"); ok && !fc {
		report("flow control ack policy requires flow control")
	}
	if hb, ok := c.dur("IdleHeartbeat"); ok && hb != time.Second {
		report("flow control ack policy heartbeat needs to be 1s")
	}
	if v, ok := c.num("MaxAckPending"); ok && v <= 0 {
		report("flow control ack policy requires max ack pending")
	}
	aw, awOK := c.dur("AckWait")
	_, n, bkOK := c.durs("BackOff")
	if awOK && aw != 0 || bkOK && n > 0 {
		report("flow control ack policy requires unset ack wait")
	}
	if v, ok := c.num("MaxDeliver"); ok && v > 0 {
		report("flow control ack policy requires unset max deliver")
	}
}

func checkBackOff(c *cfg, report func(string)) {
	_, n, ok := c.durs("BackOff")
	if !ok || n == 0 {
		return
	}
	if md, ok := c.num("MaxDeliver"); ok && md > 0 && n > int(md) {
		report("max deliver is required to be > length of backoff values")
	}
}

func checkDescription(c *cfg, report func(string)) {
	if v, ok := c.str("Description"); ok && len(v) > maxDescription {
		report(fmt.Sprintf("consumer description is too long, maximum allowed is %d", maxDescription))
	}
}

func checkPush(c *cfg, report func(string)) {
	push, _ := c.mode()
	if !push {
		return
	}
	ds, _ := c.str("DeliverSubject")
	if !natsapi.SubjectIsLiteral(ds) {
		report("consumer deliver subject has wildcards")
	}
	if !natsapi.IsValidSubject(ds) {
		report("invalid push consumer deliver subject")
	}
	if v, ok := c.num("MaxWaiting"); ok && v != 0 {
		report("consumer in push mode can not set max waiting")
	}
	if v, ok := c.num("MaxAckPending"); ok && v > 0 {
		if p, ok := c.enum("AckPolicy", "AckExplicitPolicy"); ok && p == "AckNonePolicy" {
			report("consumer requires ack policy for max ack pending")
		}
	}
	if hb, ok := c.dur("IdleHeartbeat"); ok && hb > 0 && hb < 100*time.Millisecond {
		report("consumer idle heartbeat needs to be >= 100ms")
	}
}

func checkPull(c *cfg, report func(string)) {
	_, pull := c.mode()
	if !pull {
		return
	}
	if v, ok := c.num("RateLimit"); ok && v > 0 {
		report("consumer in pull mode can not have rate limit set")
	}
	if v, ok := c.num("MaxWaiting"); ok && v < 0 {
		report("consumer max waiting needs to be positive")
	}
	if hb, ok := c.dur("IdleHeartbeat"); ok && hb > 0 {
		report("consumer idle heartbeat requires a push based consumer")
	}
	if fc, ok := c.boolean("FlowControl"); ok && fc {
		report("consumer flow control requires a push based consumer")
	}
	if v, ok := c.num("MaxRequestBatch"); ok && v < 0 {
		report("consumer max request batch needs to be > 0")
	}
	if v, ok := c.dur("MaxRequestExpires"); ok && v != 0 && v < time.Millisecond {
		report("consumer max request expires needs to be >= 1ms")
	}
}

func checkFilters(c *cfg, report func(string)) {
	single, singleOK := c.str("FilterSubject")
	multi, complete, _ := c.strs("FilterSubjects")
	if n, ok := c.sliceLen("FilterSubjects"); singleOK && single != "" && ok && n > 0 {
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
	policy, ok := c.enum("DeliverPolicy", "DeliverAllPolicy")
	if !ok {
		return
	}
	seq, seqOK := c.num("OptStartSeq")
	tm := c.ptr("OptStartTime")
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
		if tm == ptrSet {
			badStart(names[policy], "time")
		}
		if policy == "DeliverLastPerSubjectPolicy" {
			single, singleOK := c.str("FilterSubject")
			n, nOK := c.sliceLen("FilterSubjects")
			if singleOK && single == "" && nOK && n == 0 {
				notSet("last per subject", "filter subject")
			}
		}
	case "DeliverByStartSequencePolicy":
		if seqOK && seq == 0 {
			notSet("by start sequence", "start sequence")
		}
		if tm == ptrSet {
			badStart("by start sequence", "time")
		}
	case "DeliverByStartTimePolicy":
		if tm == ptrNil {
			notSet("by start time", "start time")
		}
		if seqOK && seq != 0 {
			badStart("by start time", "start sequence")
		}
	}
}

func checkSampling(c *cfg, report func(string)) {
	v, ok := c.str("SampleFrequency")
	if !ok || v == "" {
		return
	}
	n, err := strconv.Atoi(strings.TrimSuffix(v, "%"))
	if err != nil || n < 0 {
		report("failed to parse consumer sampling configuration")
	}
}

func checkFlowControlHeartbeat(c *cfg, report func(string)) {
	fc, ok := c.boolean("FlowControl")
	if !ok || !fc {
		return
	}
	if hb, ok := c.dur("IdleHeartbeat"); ok && hb == 0 {
		report("consumer with flow control also needs heartbeats")
	}
}

func checkDurableName(c *cfg, report func(string)) {
	d, dOK := c.str("Durable")
	n, nOK := c.str("Name")
	if dOK && nOK && d != "" && n != "" && d != n {
		report("Consumer Durable and Name have to be equal if both are provided")
	}
}

func checkPriority(c *cfg, report func(string)) {
	policy, ok := c.enum("PriorityPolicy", "PriorityPolicyNone")
	if !ok {
		return
	}
	groups, _, _ := c.strs("PriorityGroups")
	nGroups, groupsOK := c.sliceLen("PriorityGroups")
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
	if v, ok := c.dur("PinnedTTL"); ok && v > 0 {
		report("PinnedTTL cannot be set when PriorityPolicy is none")
	}
}
