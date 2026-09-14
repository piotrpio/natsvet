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

// Package syncsub reports NextMsg on a subscription that was not created as
// a synchronous one.
package syncsub

import (
	"fmt"
	"go/ast"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

const doc = `syncsub: report NextMsg on a subscription that is not synchronous

nats.go's validateNextMsgState returns ErrSyncSubRequired when the
subscription has a callback and ErrTypeSubscription when it is a legacy pull
subscription; for a channel subscription NextMsg silently reads from the
caller's own channel and competes with it. The rule follows the receiver
back to the single subscribe call that defined it in the same function.

	sub, _ := nc.Subscribe("s", handler)
	msg, err := sub.NextMsg(time.Second)   // always ErrSyncSubRequired`

const name = "syncsub"

var Analyzer = &analysis.Analyzer{
	Name:     name,
	Doc:      doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

type kind int

const (
	callback kind = iota
	channel
	pull
)

type ctor struct {
	recv string
	name string
	kind kind
}

// ctors lists the subscribe constructors of package nats (on Conn and on
// the legacy JetStream interface) that do not produce a sync subscription.
var ctors = []ctor{
	{"Conn", "Subscribe", callback}, {"Conn", "QueueSubscribe", callback},
	{"Conn", "ChanSubscribe", channel}, {"Conn", "ChanQueueSubscribe", channel}, {"Conn", "QueueSubscribeSyncWithChan", channel},
	{"JetStream", "Subscribe", callback}, {"JetStream", "QueueSubscribe", callback},
	{"JetStream", "ChanSubscribe", channel}, {"JetStream", "ChanQueueSubscribe", channel},
	{"JetStream", "PullSubscribe", pull},
}

func run(pass *analysis.Pass) (any, error) {
	ins := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	info := pass.TypesInfo
	ins.WithStack([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node, push bool, stack []ast.Node) bool {
		if !push {
			return false
		}
		call := n.(*ast.CallExpr)
		fn := natsapi.Callee(info, call)
		if !natsapi.IsMethod(fn, natsapi.Core, "Subscription", "NextMsg") && !natsapi.IsMethod(fn, natsapi.Core, "Subscription", "NextMsgWithContext") {
			return true
		}
		sel := call.Fun.(*ast.SelectorExpr)
		id, ok := ast.Unparen(sel.X).(*ast.Ident)
		if !ok {
			return true
		}
		def, ok := natsapi.SingleDefinition(info, natsapi.EnclosingFuncBody(stack), id)
		if !ok {
			return true
		}
		defCall, ok := ast.Unparen(def).(*ast.CallExpr)
		if !ok {
			return true
		}
		defFn := natsapi.Callee(info, defCall)
		for _, c := range ctors {
			if !natsapi.IsMethod(defFn, natsapi.Core, c.recv, c.name) {
				continue
			}
			var msg string
			switch c.kind {
			case callback:
				msg = fmt.Sprintf("NextMsg on a subscription created with %s (callback) always returns ErrSyncSubRequired; use %sSync", c.name, c.name)
			case channel:
				msg = fmt.Sprintf("NextMsg on a subscription created with %s steals messages from the channel; read the channel or use SubscribeSync", c.name)
			case pull:
				msg = "NextMsg on a pull subscription returns ErrTypeSubscription; use Fetch"
			}
			pass.Report(analysis.Diagnostic{Pos: call.Pos(), End: call.End(), Category: name, Message: msg})
			return true
		}
		return true
	})
	return nil, nil
}
