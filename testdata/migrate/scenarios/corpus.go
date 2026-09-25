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

package scenarios

import (
	"context"

	"github.com/nats-io/nats.go"
)

// The shapes below come from the corpus repositories.

// Traced sets a client trace on the handle (natscli).
func Traced(nc *nats.Conn) error {
	js, err := nc.JetStream(&nats.ClientTrace{})
	if err != nil {
		return err
	}
	return js.DeleteStream("T")
}

// Retention sets an enum field of a legacy config after building it
// (eventing-natss).
func Retention(ctx context.Context, js nats.JetStreamContext) error {
	cfg := nats.StreamConfig{Name: "R"}
	cfg.Retention = nats.WorkQueuePolicy
	_, err := js.AddStream(&cfg)
	return err
}

// Infos collects consumer infos in a map it makes (eventing-natss).
func Infos(ctx context.Context, js nats.JetStreamContext) (map[string]*nats.ConsumerInfo, error) {
	out := make(map[string]*nats.ConsumerInfo)
	info, err := js.ConsumerInfo("ORDERS", "w")
	if err != nil {
		return nil, err
	}
	out[info.Name] = info
	return out, nil
}

// watcher implements the legacy KeyWatcher, as generated mocks do
// (go-choria).
type watcher struct{}

func (watcher) Context() context.Context                 { return nil }
func (watcher) Updates() <-chan nats.KeyValueEntry       { return nil }
func (watcher) Stop() error                              { return nil }
func (watcher) Error() <-chan error                      { return nil }
func (w watcher) Watch(kv nats.KeyValue) nats.KeyWatcher { return w }

// Asserted takes a handle out of an interface value (go-choria).
func Asserted(v any) error {
	js, _ := v.(nats.JetStreamContext)
	return js.DeleteStream("A")
}

// fakeJS stands in for a handle, as test doubles do (eventing-natss).
type fakeJS struct{ nats.JetStreamContext }

func useHandle(js nats.JetStreamContext) error {
	return js.DeleteStream("F")
}

// Faked passes a stand-in to a function taking a handle.
func Faked() error {
	return useHandle(fakeJS{})
}

type framework struct{ nc *nats.Conn }

// KV returns a handle, as go-choria's Framework.KV does.
func (f framework) KV(bucket string) (nats.KeyValue, error) {
	js, err := f.nc.JetStream()
	if err != nil {
		return nil, err
	}
	return js.KeyValue(bucket)
}

// Helper gets its handle from a helper function.
func Helper(f framework) ([]byte, error) {
	kv, err := f.KV("config")
	if err != nil {
		return nil, err
	}
	e, err := kv.Get("a")
	if err != nil {
		return nil, err
	}
	return e.Value(), nil
}
