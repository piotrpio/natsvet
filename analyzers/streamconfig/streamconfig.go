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

// Package streamconfig reports stream configurations that nats-server
// rejects at create or update time.
package streamconfig

import (
	"fmt"
	"go/ast"
	"strings"
	"time"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

const doc = `streamconfig: report stream configurations the server rejects

Every check mirrors one in nats-server's checkStreamCfgLocked and fires only
when the involved fields are constants in the same composite literal, so a
report is a guaranteed runtime error, reported at the literal instead of as
a JetStreamError from CreateStream.

	jetstream.StreamConfig{
		Name:     "ORDERS",
		Subjects: []string{"orders.>", "orders.new"}, // overlap: rejected
	}`

const name = "streamconfig"

var Analyzer = &analysis.Analyzer{
	Name:     name,
	Doc:      doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

var (
	jsConfig     = natsapi.TypeRef{Pkg: natsapi.JetStream, Name: "StreamConfig"}
	legacyConfig = natsapi.TypeRef{Pkg: natsapi.Core, Name: "StreamConfig"}
)

const (
	maxName        = 255
	maxDescription = 4096
	maxReplicas    = 5
)

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	masker := natsapi.NewMasker(pass.TypesInfo)
	ins.WithStack([]ast.Node{(*ast.CompositeLit)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return false
		}
		lit := n.(*ast.CompositeLit)
		fields, matched, ok := natsapi.CompositeFields(pass.TypesInfo, lit, jsConfig, legacyConfig)
		if !ok {
			return true
		}
		c := natsapi.NewFields(pass.TypesInfo, fields, matched == legacyConfig, nil, masker.Masked(stack))
		report := func(msg string) {
			pass.Report(analysis.Diagnostic{
				Pos:      lit.Pos(),
				End:      lit.End(),
				Category: name,
				Message:  "stream config: " + msg,
			})
		}
		for _, check := range checks {
			check(c, report)
		}
		return true
	})
	return nil, nil
}

var checks = []func(c *natsapi.Fields, report func(string)){
	checkName,
	checkDescription,
	checkReplicas,
	checkWindows,
	checkRollup,
	checkCounter,
	checkDiscardNewPerSubject,
	checkDeleteMarkerTTL,
	checkSchedules,
	checkPersistMode,
	checkMirror,
	checkSubjects,
}

func invalidAssetName(s string) bool {
	return s == "" || strings.ContainsAny(s, " \t\r\n\f.*>\\/")
}

// present reports whether a field is set in the literal at all.
func present(c *natsapi.Fields, field string) bool {
	_, ok := c.Expr(field)
	return ok
}

func checkName(c *natsapi.Fields, report func(string)) {
	if !present(c, "Name") {
		return
	}
	v, ok := c.Str("Name")
	if !ok {
		return
	}
	if invalidAssetName(v) {
		report(`stream name is required and can not contain '.', '*', '>', '\', '/' or whitespace`)
	}
	if len(v) > maxName {
		report(fmt.Sprintf("stream name is too long, maximum allowed is %d", maxName))
	}
}

func checkDescription(c *natsapi.Fields, report func(string)) {
	if v, ok := c.Str("Description"); ok && len(v) > maxDescription {
		report(fmt.Sprintf("stream description is too long, maximum allowed is %d", maxDescription))
	}
}

func checkReplicas(c *natsapi.Fields, report func(string)) {
	v, ok := c.Int("Replicas")
	if !ok {
		return
	}
	if v > maxReplicas {
		report(fmt.Sprintf("maximum replicas is %d", maxReplicas))
	}
	if v < 0 {
		report("replicas count cannot be negative")
	}
}

func checkWindows(c *natsapi.Fields, report func(string)) {
	maxAge, ageOK := c.Dur("MaxAge")
	if ageOK && maxAge < 0 {
		report("max age can not be negative")
	}
	if ageOK && maxAge > 0 && maxAge < 100*time.Millisecond {
		report("max age needs to be >= 100ms")
	}
	dup, dupOK := c.Dur("Duplicates")
	if !dupOK {
		return
	}
	if dup < 0 {
		report("duplicates window can not be negative")
	}
	if ageOK && maxAge > 0 && dup > maxAge {
		report("duplicates window can not be larger then max age")
	}
	if dup > 0 && dup < 100*time.Millisecond {
		report("duplicates window needs to be >= 100ms")
	}
}

func checkRollup(c *natsapi.Fields, report func(string)) {
	deny, dOK := c.Bool("DenyPurge")
	rollup, rOK := c.Bool("AllowRollup")
	if dOK && rOK && deny && rollup {
		report("roll-ups require the purge permission")
	}
}

func checkCounter(c *natsapi.Fields, report func(string)) {
	if v, ok := c.Bool("AllowMsgCounter"); !ok || !v {
		return
	}
	if d, ok := c.Enum("Discard", "DiscardOld"); ok && d == "DiscardNew" {
		report("counter stream cannot use discard new")
	}
	if v, ok := c.Bool("AllowMsgTTL"); ok && v {
		report("counter stream cannot use message TTLs")
	}
	if v, ok := c.Bool("AllowMsgSchedules"); ok && v {
		report("counter stream cannot use message schedules")
	}
	if r, ok := c.Enum("Retention", "LimitsPolicy"); ok && r != "LimitsPolicy" {
		report("counter stream can only use limits retention")
	}
}

func checkDiscardNewPerSubject(c *natsapi.Fields, report func(string)) {
	if v, ok := c.Bool("DiscardNewPerSubject"); !ok || !v {
		return
	}
	if d, ok := c.Enum("Discard", "DiscardOld"); ok && d != "DiscardNew" {
		report("discard new per subject requires discard new policy to be set")
	}
	if n, ok := c.Int("MaxMsgsPerSubject"); ok && n <= 0 {
		report("discard new per subject requires max msgs per subject > 0")
	}
}

func checkDeleteMarkerTTL(c *natsapi.Fields, report func(string)) {
	v, ok := c.Dur("SubjectDeleteMarkerTTL")
	if !ok {
		return
	}
	if v < 0 {
		report("subject delete marker TTL must not be negative")
	}
	if v > 0 && v < time.Second {
		report("subject delete marker TTL must be at least 1 second")
	}
}

func checkSchedules(c *natsapi.Fields, report func(string)) {
	if v, ok := c.Bool("AllowMsgSchedules"); !ok || !v {
		return
	}
	if d, ok := c.Enum("Discard", "DiscardOld"); ok && d == "DiscardNew" {
		report("message scheduling cannot use discard new")
	}
	if n, ok := c.SliceLen("Sources"); ok && n > 0 {
		report("stream source can not also schedule messages")
	}
}

func checkPersistMode(c *natsapi.Fields, report func(string)) {
	if m, ok := c.Enum("PersistMode", "DefaultPersistMode"); !ok || m != "AsyncPersistMode" {
		return
	}
	if s, ok := c.Enum("Storage", "FileStorage"); ok && s != "FileStorage" {
		report("async persist mode is only supported on file storage")
	}
	if r, ok := c.Int("Replicas"); ok && r > 1 {
		report("async persist mode is not supported on replicated streams")
	}
	if v, ok := c.Bool("AllowAtomicPublish"); ok && v {
		report("async persist mode is not supported with atomic batch publish")
	}
}

func checkMirror(c *natsapi.Fields, report func(string)) {
	if c.Ptr("Mirror") != natsapi.PtrSet {
		return
	}
	if v, ok := c.Int("FirstSeq"); ok && v > 0 {
		report("stream mirrors can not have first sequence configured")
	}
	if n, ok := c.SliceLen("Subjects"); ok && n > 0 {
		report("stream mirrors can not contain subjects")
	}
	if n, ok := c.SliceLen("Sources"); ok && n > 0 {
		report("stream mirrors can not also contain other sources")
	}
	for _, f := range []struct{ field, msg string }{
		{"AllowMsgCounter", "stream mirrors can not also calculate counters"},
		{"AllowAtomicPublish", "stream mirrors can not also use atomic publishing"},
		{"AllowBatchPublish", "stream mirrors can not also use batch publishing"},
		{"AllowMsgSchedules", "stream mirrors can not also schedule messages"},
	} {
		if v, ok := c.Bool(f.field); ok && v {
			report(f.msg)
		}
	}
	if v, ok := c.Dur("SubjectDeleteMarkerTTL"); ok && v > 0 {
		report("subject delete markers forbidden on mirrors")
	}
}

func checkSubjects(c *natsapi.Fields, report func(string)) {
	subjects, _, known := c.Strs("Subjects")
	if !known || len(subjects) == 0 {
		return
	}
	noAck, noAckOK := c.Bool("NoAck")
	replicas, replicasOK := c.Int("Replicas")
	seen := make(map[string]bool, len(subjects))
	for i, subj := range subjects {
		if !natsapi.IsValidSubject(subj) {
			report(fmt.Sprintf("invalid subject %q", subj))
			continue
		}
		if seen[subj] {
			report("duplicate subjects detected")
			continue
		}
		if subj == ">" {
			if noAckOK && !noAck {
				report("capturing all subjects requires no-ack to be true")
			}
			if replicasOK && replicas > 1 {
				report("capturing all subjects requires replicas of 1")
			}
		} else if noAckOK && !noAck {
			for _, ns := range []string{"$JS.>", "$JSC.>", "$NRG.>"} {
				if natsapi.SubjectsCollide(subj, ns) && !natsapi.SubjectIsSubsetMatch(subj, "$JS.EVENT.>") {
					report("subjects that overlap with jetstream api require no-ack to be true")
					break
				}
			}
			if natsapi.SubjectsCollide(subj, "$SYS.>") && !natsapi.SubjectIsSubsetMatch(subj, "$SYS.ACCOUNT.>") {
				report("subjects that overlap with system api require no-ack to be true")
			}
		}
		for _, other := range subjects[i+1:] {
			if other != subj && natsapi.IsValidSubject(other) && natsapi.SubjectsCollide(other, subj) {
				report(fmt.Sprintf("subject %q overlaps with %q", subj, other))
			}
		}
		seen[subj] = true
	}
}
