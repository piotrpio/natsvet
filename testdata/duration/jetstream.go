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
package duration

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const timeout = 5

var n int64

func calls(nc *nats.Conn, sub *nats.Subscription, js jetstream.JetStream, c jetstream.Consumer) {
	_, _ = nc.Request("s", nil, 5)                                                             // want `duration 5 for Request is 5ns; multiply by a time unit such as time.Second or time.Millisecond`
	_, _ = nc.Request("s", nil, timeout)                                                       // want `duration 5 for Request is 5ns`
	_, _ = nc.Request("s", nil, 2*5)                                                           // want `duration 10 for Request is 10ns`
	_, _ = nats.Connect("", nats.Timeout(10))                                                  // want `duration 10 for Timeout is 10ns`
	_, _ = nats.Connect("", nats.ReconnectJitter(1, 2))                                        // want `duration 1 for ReconnectJitter is 1ns` `duration 2 for ReconnectJitter is 2ns`
	_, _ = sub.NextMsg(500)                                                                    // want `duration 500 for NextMsg is 500ns`
	_, _ = c.Fetch(1, jetstream.FetchMaxWait(30))                                              // want `duration 30 for FetchMaxWait is 30ns`
	_ = nc.FlushTimeout(1000)                                                                  // want `duration 1000 for FlushTimeout is 1µs`
	_, _ = js.CreateConsumer(context.Background(), "S", jetstream.ConsumerConfig{AckWait: 30}) // want `duration 30 for AckWait is 30ns`

	_, _ = nc.Request("s", nil, 500*time.Microsecond)
	_, _ = nc.Request("s", nil, 5*time.Second)
	_, _ = nc.Request("s", nil, time.Duration(n))
	_, _ = nc.Request("s", nil, nats.DefaultTimeout)
	_, _ = nc.Request("s", nil, 2*nats.DefaultTimeout)
	_, _ = sub.NextMsg(0)
	_ = sub.AutoUnsubscribe(5)
	time.Sleep(5)
	userDuration(5)
}

func userDuration(d time.Duration) {}

var _ = []jetstream.ConsumerConfig{
	{AckWait: 30},                    // want `duration 30 for AckWait is 30ns`
	{BackOff: []time.Duration{1, 2}}, // want `duration 1 for BackOff is 1ns` `duration 2 for BackOff is 2ns`
	{MaxRequestExpires: 86400000},    // want `duration 86400000 for MaxRequestExpires is 86.4ms`
	{AckWait: 500 * time.Millisecond},
	{AckWait: 0},
	{InactiveThreshold: -1},
	{BackOff: []time.Duration{time.Second, 2 * time.Second}},
}

var _ = jetstream.StreamConfig{MaxAge: 86400000} // want `duration 86400000 for MaxAge is 86.4ms`

var _ = jetstream.KeyValueConfig{Bucket: "b", TTL: 3600} // want `duration 3600 for TTL is 3.6µs`

func assignments() {
	opts := nats.GetDefaultOptions()
	opts.Timeout = 5 // want `duration 5 for Timeout is 5ns`
	opts.ReconnectWait = 2 * time.Second
	cfg := jetstream.ConsumerConfig{}
	cfg.AckWait = 30 // want `duration 30 for AckWait is 30ns`
	cfg.AckWait = 30 * time.Second
	cfg.MaxDeliver = 5
	_, _ = opts, cfg
}
