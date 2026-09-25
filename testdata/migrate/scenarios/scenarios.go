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

// Package scenarios holds one function per planner scenario.
package scenarios

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
)

// Setup has a context in scope.
func Setup(ctx context.Context, js nats.JetStreamContext, cfg nats.StreamConfig) error {
	_, err := js.AddStream(&cfg)
	return err
}

type request struct{ ctx context.Context }

// Create passes its context through nats.Context and has none in scope.
func (r request) Create(js nats.JetStreamContext, cfg nats.StreamConfig) error {
	_, err := js.AddStream(&cfg, nats.Context(r.ctx))
	return err
}

// Drop has no context anywhere.
func Drop(js nats.JetStreamContext) error {
	return js.DeleteStream("S")
}

// Timed bounds one call with MaxWait.
func Timed(js nats.JetStreamContext) error {
	return js.DeleteStream("S", nats.MaxWait(time.Second))
}

// Consumer creates a consumer whose config renames a field and relies on
// the legacy zero ack policy.
func Consumer(ctx context.Context, js nats.JetStreamContext) error {
	_, err := js.AddConsumer("ORDERS", &nats.ConsumerConfig{Durable: "w", DeliverSubject: "w.deliver", Heartbeat: time.Second})
	return err
}

// Name resolves a stream by subject.
func Name(js nats.JetStreamContext) (string, error) {
	return js.StreamNameBySubject("orders.new")
}

// Bound pulls from a named stream with options folded into the config.
func Bound(js nats.JetStreamContext) error {
	sub, err := js.PullSubscribe("orders.new", "", nats.Durable("w"), nats.BindStream("ORDERS"), nats.DeliverNew(), nats.MaxAckPending(100))
	if err != nil {
		return err
	}
	defer sub.Drain()
	return nil
}

// Pull creates a pull consumer on an unbound subject.
func Pull(js nats.JetStreamContext) error {
	sub, err := js.PullSubscribe("orders.new", "w")
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	return nil
}

// Billing subscribes to a subject no stream in the module covers.
func Billing(ctx context.Context, js nats.JetStreamContext) error {
	_, err := js.Subscribe("billing.new", func(m *nats.Msg) {}, nats.Durable("billing"))
	return err
}

// Buckets creates a KeyValue bucket and reads from it.
func Buckets(nc *nats.Conn) ([]byte, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}
	kv, err := js.CreateKeyValue(&nats.KeyValueConfig{Bucket: "settings"})
	if err != nil {
		return nil, err
	}
	e, err := kv.Get("a")
	if err != nil {
		return nil, err
	}
	return e.Value(), nil
}

// Config builds a config in one statement and passes it in another.
func Config(ctx context.Context, js nats.JetStreamContext) error {
	cfg := nats.ConsumerConfig{Durable: "c", AckPolicy: nats.AckExplicitPolicy}
	cfg.MaxDeliver = 3
	_, err := js.AddConsumer("ORDERS", &cfg)
	return err
}

// Policy picks an ack policy at runtime.
func Policy(ctx context.Context, js nats.JetStreamContext, explicit bool) error {
	ack := nats.AckNonePolicy
	if explicit {
		ack = nats.AckExplicitPolicy
	}
	_, err := js.AddConsumer("ORDERS", &nats.ConsumerConfig{Durable: "p", AckPolicy: ack})
	return err
}

// All acks cumulatively and relied on the legacy auto-ack wrapper.
func All(ctx context.Context, js nats.JetStreamContext) error {
	_, err := js.Subscribe("orders.all", func(m *nats.Msg) {}, nats.Durable("all"), nats.AckAll())
	return err
}
