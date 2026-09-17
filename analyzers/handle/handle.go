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

// Package handle reports a lifecycle handle that is thrown away at the call
// that created it.
package handle

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

const doc = `handle: report a discarded ConsumeContext, MessagesContext, watcher or micro.Service

The handle returned by Consume, Messages, Watch or AddService is the only way
to Stop or Drain what the call started; assigned to the blank identifier or
dropped as an expression statement, the consumer, watcher or service runs
until the connection closes and in-flight work cannot be finished cleanly. A
Consume or Messages call with a jetstream.StopAfter option stops itself and
is not reported.

	_, err := cons.Consume(handler)   // nothing can ever stop this consumer
	cc, err := cons.Consume(handler)
	defer cc.Drain()`

const name = "handle"

var Analyzer = &analysis.Analyzer{
	Name:     name,
	Doc:      doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

type hook struct {
	pkg    natsapi.Pkg
	recv   string // "" for a package-level function
	method string
	typ    string
	tail   string
}

const (
	consumerTail = "the consumer can never be stopped or drained"
	watcherTail  = "the watcher can never be stopped and its subscription lives as long as the connection"
	serviceTail  = "the service can never be stopped and keeps answering until the connection closes"
)

var hooks = []hook{
	{natsapi.JetStream, "Consumer", "Consume", "ConsumeContext", consumerTail},
	{natsapi.JetStream, "PushConsumer", "Consume", "ConsumeContext", consumerTail},
	{natsapi.JetStream, "Consumer", "Messages", "MessagesContext", consumerTail},
	{natsapi.JetStream, "KeyValue", "Watch", "KeyWatcher", watcherTail},
	{natsapi.JetStream, "KeyValue", "WatchAll", "KeyWatcher", watcherTail},
	{natsapi.JetStream, "KeyValue", "WatchFiltered", "KeyWatcher", watcherTail},
	{natsapi.JetStream, "ObjectStore", "Watch", "ObjectWatcher", watcherTail},
	{natsapi.Core, "KeyValue", "Watch", "KeyWatcher", watcherTail},
	{natsapi.Core, "KeyValue", "WatchAll", "KeyWatcher", watcherTail},
	{natsapi.Core, "KeyValue", "WatchFiltered", "KeyWatcher", watcherTail},
	{natsapi.Core, "ObjectStore", "Watch", "ObjectWatcher", watcherTail},
	{natsapi.Micro, "", "AddService", "Service", serviceTail},
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
		for _, h := range hooks {
			if h.recv == "" && !natsapi.IsFunc(fn, h.pkg, h.method) || h.recv != "" && !natsapi.IsMethod(fn, h.pkg, h.recv, h.method) {
				continue
			}
			if !natsapi.Discarded(stack) || selfStopping(info, call) {
				return true
			}
			pass.Report(analysis.Diagnostic{
				Pos: call.Pos(), End: call.End(), Category: name,
				Message: fmt.Sprintf("%s from %s discarded; %s", h.typ, h.method, h.tail),
			})
			return true
		}
		return true
	})
	return nil, nil
}

// selfStopping reports whether call carries a jetstream.StopAfter option.
func selfStopping(info *types.Info, call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		if natsapi.IsNamedType(info.TypeOf(arg), natsapi.JetStream, "StopAfter") {
			return true
		}
	}
	return false
}
