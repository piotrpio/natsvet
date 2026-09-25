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

func nearMisses(msg *nats.Msg, h nats.Header, id string) {
	for k := range msg.Header {
		if k == "Nats-UpTo-Sequnce" { // want `header key "Nats-UpTo-Sequnce" is not a header NATS sets or reads; the closest NATS header is "Nats-UpTo-Sequence"`
			continue
		}
	}
	h.Set("Nats-MsgId", id)                     // want `header key "Nats-MsgId" is not a header NATS sets or reads; the closest NATS header is "Nats-Msg-Id"`
	_ = h.Get("nats-upto-sequnce")              // want `header key "nats-upto-sequnce" is not a header NATS sets or reads; the closest NATS header is "Nats-UpTo-Sequence"`
	msg.Header.Add("Nats-TTLSeconds", "30s")    // want `header key "Nats-TTLSeconds" is not a header NATS sets or reads; the closest NATS header is "Nats-TTL"`
	_ = nats.Header{"Nats-TTLSeconds": {"30s"}} // want `header key "Nats-TTLSeconds" is not a header NATS sets or reads; the closest NATS header is "Nats-TTL"`

	h.Set("Nats-Has-More", "1")
	h.Set("Nats-X", "x")
	h.Set("Data-TTL", "1")
	h.Set("Nats-TTL-Seconds", "30")
	_ = h.Get("Nats-Scheduler")
	_ = h.Get("Nats-Schedule-TTL")
}
