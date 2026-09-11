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

package legacyjs

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type holder struct {
	js nats.JetStreamContext // want `legacy JetStream API: nats.JetStreamContext; see the jetstream package`
}

func legacy(nc *nats.Conn, msg *nats.Msg, sub *nats.Subscription) {
	js, _ := nc.JetStream()                                   // want `legacy JetStream API: nats.Conn.JetStream; see the jetstream package`
	_, _ = js.Publish("orders", nil)                          // want `legacy JetStream API: nats.JetStream.Publish; see the jetstream package`
	_, _ = js.AddStream(&nats.StreamConfig{Name: "orders"})   // want `legacy JetStream API: nats.JetStreamManager.AddStream; see the jetstream package` `legacy JetStream API: nats.StreamConfig; see the jetstream package`
	_, _ = js.SubscribeSync("orders", nats.Durable("worker")) // want `legacy JetStream API: nats.JetStream.SubscribeSync; see the jetstream package` `legacy JetStream API: nats.Durable; see the jetstream package`
	_, _ = js.KeyValue("bucket")                              // want `legacy JetStream API: nats.KeyValueManager.KeyValue; see the jetstream package`
	_, _ = sub.Fetch(10, nats.MaxWait(time.Second))           // want `legacy JetStream API: nats.Subscription.Fetch; see the jetstream package` `legacy JetStream API: nats.MaxWait; see the jetstream package`
	_ = msg.Ack()                                             // want `legacy JetStream API: nats.Msg.Ack; see the jetstream package`

	var kv nats.KeyValue // want `legacy JetStream API: nats.KeyValue; see the jetstream package`
	_ = kv

	_ = nats.MsgIdHdr
	_ = nats.ErrJetStreamNotEnabled
	_ = nc.Publish("orders", nil)
	_, _ = nc.SubscribeSync("orders")
	_, _ = sub.NextMsg(time.Second)
}

func modern(nc *nats.Conn) {
	js, _ := jetstream.New(nc)
	_, _ = js.Publish(context.Background(), "orders", nil)
	_, _ = js.CreateStream(context.Background(), jetstream.StreamConfig{Name: "orders"})
	_, _ = js.KeyValue(context.Background(), "bucket")
}
