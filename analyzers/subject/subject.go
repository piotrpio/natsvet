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

// Package subject reports constant subjects and queue group names that
// nats.go or the server rejects, and publish subjects that contain
// wildcards.
package subject

import (
	"fmt"
	"go/ast"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

const doc = `subject: report invalid subjects, wildcard publishes and bad queue names

nats.go returns ErrBadSubject for an empty subject or one containing
whitespace, the server rejects a subscription with an empty token or a
misplaced '>', and a wildcard token in a publish subject is sent literally,
so the message reaches only subscriptions that spell out that literal
token. A queue group containing whitespace returns ErrBadQueueName. Only
constant subjects and queue names are examined.

	nc.Publish("orders.*", data)      // reaches nobody who subscribed to orders.*
	nc.Subscribe("foo..bar", handler) // ErrBadSubject`

const name = "subject"

var Analyzer = &analysis.Analyzer{
	Name:     name,
	Doc:      doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// methodHook describes the subject-bearing arguments of a method: the
// subject index, an optional reply index (-1 when none), an optional queue
// group index (-1 when none), and whether the subject is a publish subject.
type methodHook struct {
	pkg     natsapi.Pkg
	recv    string
	name    string
	subj    int
	reply   int
	queue   int
	publish bool
	// emptyOK marks the legacy JetStream subscribe methods, where an empty
	// subject is valid when a stream is bound with Bind or BindStream
	// (js.go: "subject required" only without a stream).
	emptyOK bool
}

var methodHooks = func() []methodHook {
	var hs []methodHook
	add := func(pkg natsapi.Pkg, recv string, subj, reply, queue int, publish bool, names ...string) {
		for _, n := range names {
			hs = append(hs, methodHook{pkg, recv, n, subj, reply, queue, publish, pkg == natsapi.Core && recv == "JetStream" && !publish})
		}
	}
	add(natsapi.Core, "Conn", 0, -1, -1, true, "Publish", "Request")
	add(natsapi.Core, "Conn", 0, 1, -1, true, "PublishRequest")
	add(natsapi.Core, "Conn", 1, -1, -1, true, "RequestWithContext")
	add(natsapi.Core, "Conn", 0, -1, -1, false, "Subscribe", "SubscribeSync", "ChanSubscribe")
	add(natsapi.Core, "Conn", 0, -1, 1, false, "QueueSubscribe", "QueueSubscribeSync", "ChanQueueSubscribe", "QueueSubscribeSyncWithChan")
	add(natsapi.JetStream, "Publisher", 1, -1, -1, true, "Publish")
	add(natsapi.JetStream, "Publisher", 0, -1, -1, true, "PublishAsync")
	add(natsapi.Core, "JetStream", 0, -1, -1, true, "Publish", "PublishAsync")
	add(natsapi.Core, "JetStream", 0, -1, -1, false, "Subscribe", "SubscribeSync", "ChanSubscribe", "PullSubscribe")
	add(natsapi.Core, "JetStream", 0, -1, 1, false, "QueueSubscribe", "QueueSubscribeSync", "ChanQueueSubscribe")
	return hs
}()

type funcHook struct {
	pkg     natsapi.Pkg
	name    string
	publish bool
}

var funcHooks = []funcHook{
	{natsapi.Core, "NewMsg", true},
	{natsapi.Micro, "WithEndpointSubject", false},
}

type fieldHook struct {
	field   string
	publish bool
	queue   bool
}

var fieldHooks = map[natsapi.TypeRef][]fieldHook{
	{Pkg: natsapi.Core, Name: "Msg"}:             {{"Subject", true, false}, {"Reply", true, false}},
	{Pkg: natsapi.Micro, Name: "EndpointConfig"}: {{"Subject", false, false}, {"QueueGroup", false, true}},
}

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	info := pass.TypesInfo
	report := func(e ast.Expr, msg string) {
		pass.Report(analysis.Diagnostic{Pos: e.Pos(), End: e.End(), Category: name, Message: msg})
	}
	checkSubject := func(e ast.Expr, publish, emptyOK bool) {
		s, ok := natsapi.ConstString(info, e)
		if !ok || s == "" && emptyOK {
			return
		}
		if reason, bad := subjectInvalid(s); bad {
			report(e, fmt.Sprintf("subject %q is invalid: %s", s, reason))
			return
		}
		if publish && !natsapi.SubjectIsLiteral(s) {
			report(e, fmt.Sprintf("publish subject %q contains a wildcard; wildcards only match in subscriptions", s))
		}
	}
	checkQueue := func(e ast.Expr) {
		if q, ok := natsapi.ConstString(info, e); ok && strings.ContainsAny(q, " \t\r\n") {
			report(e, fmt.Sprintf("queue group %q contains whitespace (ErrBadQueueName)", q))
		}
	}
	ins.Preorder([]ast.Node{(*ast.CallExpr)(nil), (*ast.CompositeLit)(nil)}, func(n ast.Node) {
		switch n := n.(type) {
		case *ast.CallExpr:
			fn := natsapi.Callee(info, n)
			if fn == nil {
				return
			}
			for _, h := range funcHooks {
				if natsapi.IsFunc(fn, h.pkg, h.name) && len(n.Args) > 0 {
					checkSubject(n.Args[0], h.publish, false)
					return
				}
			}
			for _, h := range methodHooks {
				if !natsapi.IsMethod(fn, h.pkg, h.recv, h.name) {
					continue
				}
				if h.subj < len(n.Args) {
					checkSubject(n.Args[h.subj], h.publish, h.emptyOK)
				}
				if h.reply >= 0 && h.reply < len(n.Args) {
					checkSubject(n.Args[h.reply], h.publish, false)
				}
				if h.queue >= 0 && h.queue < len(n.Args) {
					checkQueue(n.Args[h.queue])
				}
				return
			}
		case *ast.CompositeLit:
			for ref, hooks := range fieldHooks {
				fields, _, ok := natsapi.CompositeFields(info, n, ref)
				if !ok {
					continue
				}
				for _, h := range hooks {
					e, present := fields[h.field]
					if !present {
						continue
					}
					if h.queue {
						checkQueue(e)
					} else {
						checkSubject(e, h.publish, false)
					}
				}
				return
			}
		}
	})
	return nil, nil
}

// subjectInvalid mirrors nats.go validateSubject (empty, whitespace) and
// nats-server IsValidSubject (empty token, '>' placement), in that order.
func subjectInvalid(s string) (string, bool) {
	switch {
	case s == "":
		return "empty subject", true
	case strings.ContainsAny(s, " \t\r\n\f"):
		return "contains whitespace", true
	}
	toks := strings.Split(s, ".")
	for i, t := range toks {
		if t == "" {
			return "empty token", true
		}
		if t == ">" && i != len(toks)-1 {
			return "'>' must be the last token", true
		}
	}
	if !natsapi.IsValidSubject(s) {
		return "invalid subject", true
	}
	return "", false
}
