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

package handle

import (
	"context"

	"github.com/nats-io/nats.go/jetstream"
)

var (
	ctx     = context.Background()
	handler jetstream.MessageHandler
	n       int
)

func consumeBlank(cons jetstream.Consumer) {
	_, err := cons.Consume(handler) // want `ConsumeContext from Consume discarded; the consumer can never be stopped or drained`
	_ = err
}

func consumeExprStmt(cons jetstream.Consumer) {
	cons.Consume(handler) // want `ConsumeContext from Consume discarded; the consumer can never be stopped or drained`
}

func pushConsumer(pc jetstream.PushConsumer) {
	var err error
	_, err = pc.Consume(handler) // want `ConsumeContext from Consume discarded; the consumer can never be stopped or drained`
	_ = err
}

func messagesIfInit(cons jetstream.Consumer) {
	if _, err := cons.Messages(); err != nil { // want `MessagesContext from Messages discarded; the consumer can never be stopped or drained`
		return
	}
}

func handleKept(cons jetstream.Consumer) {
	cc, err := cons.Consume(handler)
	if err != nil {
		return
	}
	defer cc.Stop()
}

func handleReturned(cons jetstream.Consumer) (jetstream.ConsumeContext, error) {
	return cons.Consume(handler)
}

func stopAfterLiteral(cons jetstream.Consumer) {
	_, err := cons.Consume(handler, jetstream.StopAfter(10))
	_ = err
}

func stopAfterVariable(cons jetstream.Consumer) {
	limit := jetstream.StopAfter(n)
	_, err := cons.Messages(limit)
	_ = err
}

func otherOptionsOnly(cons jetstream.Consumer) {
	_, err := cons.Consume(handler, jetstream.PullMaxMessages(100)) // want `ConsumeContext from Consume discarded`
	_ = err
}

func kvWatch(kv jetstream.KeyValue) {
	var err error
	_, err = kv.Watch(ctx, "orders.*")                 // want `KeyWatcher from Watch discarded; the watcher can never be stopped and its subscription lives as long as the connection`
	_, err = kv.WatchAll(ctx)                          // want `KeyWatcher from WatchAll discarded`
	_, err = kv.WatchFiltered(ctx, []string{"a", "b"}) // want `KeyWatcher from WatchFiltered discarded`
	_ = err
}

func objectWatch(os jetstream.ObjectStore) {
	_, err := os.Watch(ctx) // want `ObjectWatcher from Watch discarded; the watcher can never be stopped and its subscription lives as long as the connection`
	_ = err
}

func watcherKept(kv jetstream.KeyValue) {
	w, err := kv.Watch(ctx, "orders.*")
	if err != nil {
		return
	}
	defer w.Stop()
}

func keyLister(kv jetstream.KeyValue) {
	_, err := kv.ListKeys(ctx)
	_ = err
}

type holder struct {
	cc jetstream.ConsumeContext
}

func storedInField(s *holder, cons jetstream.Consumer) {
	var err error
	s.cc, err = cons.Consume(handler)
	_ = err
}

type stopper interface{ Stop() }

type worker struct{}

func (worker) Consume(h jetstream.MessageHandler) (stopper, error) { return nil, nil }

func userType(w worker) {
	_, err := w.Consume(handler)
	_ = err
}
