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
	"github.com/nats-io/nats.go/jetstream"
)

func both(msg *nats.Msg, m jetstream.Msg) {
	_ = msg.Header.Get("nats-msg-id")  // want `header key "nats-msg-id" does not match "Nats-Msg-Id"; nats.go header lookups are case-sensitive`
	_ = m.Headers().Get("nats-msg-id") // want `header key "nats-msg-id" does not match "Nats-Msg-Id"; nats.go header lookups are case-sensitive`
}
