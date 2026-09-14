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
	"time"

	"github.com/nats-io/nats.go"
)

var _ = nats.StreamConfig{MaxAge: 3600} // want `duration 3600 for MaxAge is 3.6µs`

var _ = nats.ConsumerConfig{AckWait: 30, Heartbeat: 5 * time.Second} // want `duration 30 for AckWait is 30ns`

func legacy(js nats.JetStreamContext, sub *nats.Subscription) {
	_, _ = js.SubscribeSync("s", nats.AckWait(30))                              // want `duration 30 for AckWait is 30ns`
	_, _ = js.SubscribeSync("s", nats.BackOff([]time.Duration{1, time.Second})) // want `duration 1 for BackOff is 1ns`
	_, _ = sub.Fetch(1, nats.MaxWait(500))                                      // want `duration 500 for MaxWait is 500ns`
	_, _ = js.SubscribeSync("s", nats.AckWait(30*time.Second))
}
