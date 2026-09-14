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

package tablegen

import (
	"strings"
	"testing"
)

const testdataDir = "../../testdata"

func load(t *testing.T) *Tables {
	t.Helper()
	if err := Available(testdataDir); err != nil {
		t.Skipf("pinned nats.go not available: %v (run `make testdata-deps`)", err)
	}
	tables, err := Collect(testdataDir)
	if err != nil {
		t.Fatal(err)
	}
	return tables
}

func TestCollectHeaders(t *testing.T) {
	tables := load(t)
	tests := []struct {
		header string
		want   []HeaderConst
	}{
		{"Nats-Msg-Id", []HeaderConst{{"jetstream", "MsgIDHeader"}, {"nats", "MsgIdHdr"}}},
		{"Nats-Service-Error", []HeaderConst{{"micro", "ErrorHeader"}}},
		{"Nats-Schedule", []HeaderConst{{"jetstream", "ScheduleHeader"}}},
	}
	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			got := tables.Headers[tt.header]
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("entry %d: got %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
	if _, ok := tables.Headers["X-Request-Id"]; ok {
		t.Error("unknown header present")
	}
	for h := range tables.Headers {
		if !strings.HasPrefix(h, "Nats-") {
			t.Errorf("header %q does not start with Nats-", h)
		}
	}
	if n := len(tables.Headers); n < 20 {
		t.Errorf("only %d headers collected", n)
	}
}

func TestCollectLegacy(t *testing.T) {
	tables := load(t)
	present := []string{
		"JetStreamContext", "JetStream", "JetStreamManager", "KeyValue", "ObjectStore",
		"JetStream.Publish", "Subscription.Fetch", "Msg.Ack", "Conn.JetStream",
		"Durable", "MaxWait", "StreamConfig", "ConsumerConfig", "KeyValueConfig",
		"JetStreamError", "PubOpt", "SubOpt",
	}
	for _, s := range present {
		if !tables.Legacy[s] {
			t.Errorf("legacy table lacks %q", s)
		}
	}
	absent := []string{
		"MsgIdHdr", "ErrJetStreamNotEnabled", "Conn.Publish", "Conn", "Msg", "Subscription",
		"Header", "Subscription.NextMsg", "Connect",
	}
	for _, s := range absent {
		if tables.Legacy[s] {
			t.Errorf("legacy table contains %q", s)
		}
	}
	if n := len(tables.Legacy); n < 200 {
		t.Errorf("only %d legacy symbols collected", n)
	}
}

func TestCollectDurations(t *testing.T) {
	tables := load(t)
	for _, s := range []string{"nats.MaxWait", "nats.AckWait", "jetstream.PullExpiry", "jetstream.PullHeartbeat"} {
		if !tables.Durations[s] {
			t.Errorf("duration table lacks %q", s)
		}
	}
	for _, s := range []string{"nats.PullMaxWaiting", "jetstream.ConsumerConfig", "nats.nakDelay"} {
		if tables.Durations[s] {
			t.Errorf("duration table contains %q", s)
		}
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	tables := load(t)
	a, err := tables.Generate()
	if err != nil {
		t.Fatal(err)
	}
	b, err := tables.Generate()
	if err != nil {
		t.Fatal(err)
	}
	if string(a.Headers) != string(b.Headers) || string(a.Legacy) != string(b.Legacy) {
		t.Error("two generations differ")
	}
	for _, src := range [][]byte{a.Headers, a.Legacy, a.Durations} {
		if !strings.HasPrefix(string(src), "// Copyright") {
			t.Error("generated file lacks the license header")
		}
		if !strings.Contains(string(src), "DO NOT EDIT") {
			t.Error("generated file lacks the generated marker")
		}
	}
}
