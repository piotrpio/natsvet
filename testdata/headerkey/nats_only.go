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

package headerkey

import (
	"github.com/nats-io/nats.go"
)

const id = "nats-msg-id"

func natsOnly(msg *nats.Msg, h nats.Header, key string) {
	_ = msg.Header.Get("nats-msg-id")           // want `header key "nats-msg-id" does not match "Nats-Msg-Id"; nats.go header lookups are case-sensitive`
	msg.Header.Set("NATS-MSG-ID", "1")          // want `header key "NATS-MSG-ID" does not match "Nats-Msg-Id"; nats.go header lookups are case-sensitive`
	msg.Header.Add("nats-expected-stream", "s") // want `header key "nats-expected-stream" does not match "Nats-Expected-Stream"; nats.go header lookups are case-sensitive`
	_ = msg.Header.Values("nats-sequence")      // want `header key "nats-sequence" does not match "Nats-Sequence"; nats.go header lookups are case-sensitive`
	msg.Header.Del("nats-stream")               // want `header key "nats-stream" does not match "Nats-Stream"; nats.go header lookups are case-sensitive`
	_ = h["nats-msg-id"]                        // want `header key "nats-msg-id" does not match "Nats-Msg-Id"; nats.go header lookups are case-sensitive`
	_ = h.Get(id)                               // want `header key "nats-msg-id" does not match "Nats-Msg-Id"; nats.go header lookups are case-sensitive`
	_ = h.Get("nats-schedule")                  // want `header key "nats-schedule" does not match "Nats-Schedule"; nats.go header lookups are case-sensitive`

	_ = h.Get("Nats-Msg-Id")
	_ = h.Get(nats.MsgIdHdr)
	h.Set("X-My-Header", "v")
	_ = h.Get("x-request-id")
	_ = h.Get(key)
	_ = h[key]
	_ = h.Get("nats-msg-id" + key)
}
