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

package ackhandler

import (
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

var data []byte

func onAck(jetstream.JetStream, *nats.Msg, *jetstream.PubAck) {}

func newJS(nc *nats.Conn) (jetstream.JetStream, error) {
	return jetstream.New(nc, jetstream.WithPublishAsyncAckHandler(onAck))
}

func discard(js jetstream.JetStream) {
	_, err := js.PublishAsync("orders.new", data) // want `PubAckFuture from PublishAsync discarded and this package never sets an async error handler or awaits PublishAsyncComplete; publish errors are lost`
	_ = err
}
