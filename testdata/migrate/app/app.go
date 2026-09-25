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

// Service holds a legacy JetStream handle used from this package and from
// package worker.
type Service struct {
	JS nats.JetStreamContext
	kv nats.KeyValue
}

func New(nc *nats.Conn) (*Service, error) {
	js, err := nc.JetStream(nats.MaxWait(2 * time.Second))
	if err != nil {
		return nil, err
	}
	kv, err := js.KeyValue("config")
	if err != nil {
		return nil, err
	}
	return &Service{JS: js, kv: kv}, nil
}

func (s *Service) Setup(ctx context.Context) error {
	if _, err := s.JS.AddStream(&nats.StreamConfig{Name: "ORDERS", Subjects: []string{"orders.>"}}); err != nil {
		return err
	}
	_, err := s.JS.AddConsumer("ORDERS", &nats.ConsumerConfig{Durable: "audit", DeliverSubject: "audit.deliver", Heartbeat: time.Second})
	return err
}

func (s *Service) Purge() error {
	return s.JS.PurgeStream("ORDERS")
}

func (s *Service) Publish(ctx context.Context, id string, data []byte) error {
	_, err := s.JS.Publish("orders.new", data, nats.MsgId(id))
	return err
}

func (s *Service) Setting(key string) (string, error) {
	e, err := s.kv.Get(key)
	if errors.Is(err, nats.ErrKeyNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return string(e.Value()), nil
}

func (s *Service) StreamNames() []string {
	var names []string
	for n := range s.JS.StreamNames() {
		names = append(names, n)
	}
	return names
}
