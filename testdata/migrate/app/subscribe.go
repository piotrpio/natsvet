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

package app

import (
	"context"
	"errors"
	"time"

	"github.com/nats-io/nats.go"
)

func process(subject string, data []byte) {}

// Run is an unbound durable callback subscription that auto-acked.
func (s *Service) Run(ctx context.Context) error {
	sub, err := s.JS.Subscribe("orders.new", func(m *nats.Msg) {
		process(m.Subject, m.Data)
	}, nats.Durable("workers"), nats.DeliverNew())
	if err != nil {
		return err
	}
	defer sub.Drain()
	<-ctx.Done()
	return nil
}

// Queue is bound to an existing push consumer and acks by itself.
func (s *Service) Queue(ctx context.Context) error {
	sub, err := s.JS.QueueSubscribe("orders.new", "q", func(m *nats.Msg) {
		process(m.Subject, m.Data)
		m.Ack()
	}, nats.Bind("ORDERS", "queue"), nats.ManualAck())
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	<-ctx.Done()
	return nil
}

// Tail is an ordered subscription.
func (s *Service) Tail(ctx context.Context) error {
	sub, err := s.JS.Subscribe("orders.>", func(m *nats.Msg) {
		process(m.Subject, m.Data)
	}, nats.OrderedConsumer(), nats.DeliverLast())
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	<-ctx.Done()
	return nil
}

// Limited uses a push-only option on an unbound subscription.
func (s *Service) Limited(ctx context.Context) error {
	sub, err := s.JS.Subscribe("orders.limited", func(m *nats.Msg) {
		process(m.Subject, m.Data)
	}, nats.ConsumerName("limited"), nats.RateLimit(1024))
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()
	<-ctx.Done()
	return nil
}

// Batch pulls with a Fetch loop.
func (s *Service) Batch() error {
	sub, err := s.JS.PullSubscribe("orders.new", "batch")
	if err != nil {
		return err
	}
	msgs, err := sub.Fetch(10, nats.MaxWait(time.Second))
	if errors.Is(err, nats.ErrTimeout) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, m := range msgs {
		process(m.Subject, m.Data)
		m.Ack()
	}
	return nil
}

// Next reads a synchronous subscription.
func (s *Service) Next() ([]byte, error) {
	sub, err := s.JS.SubscribeSync("orders.sync", nats.Durable("sync"))
	if err != nil {
		return nil, err
	}
	m, err := sub.NextMsg(time.Second)
	if err != nil {
		return nil, err
	}
	return m.Data, m.Ack()
}

// Channel delivers into a buffered channel.
func (s *Service) Channel(ch chan *nats.Msg) error {
	_, err := s.JS.ChanSubscribe("orders.chan", ch, nats.Durable("chan"))
	return err
}

// handle is shared by a core and a JetStream subscription.
func handle(m *nats.Msg) {
	process(m.Subject, m.Data)
}

func (s *Service) Shared(nc *nats.Conn) error {
	if _, err := nc.Subscribe("core.events", handle); err != nil {
		return err
	}
	_, err := s.JS.Subscribe("orders.shared", handle, nats.Durable("shared"))
	return err
}
