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

package subject

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go"
)

const prefix = "orders"

var (
	subj    string
	id      string
	handler nats.MsgHandler
	ch      chan *nats.Msg
)

func core(nc *nats.Conn) {
	_, _ = nc.Subscribe("foo..bar", handler)                 // want `subject "foo..bar" is invalid: empty token`
	_ = nc.Publish("foo. bar", nil)                          // want `subject "foo. bar" is invalid: contains whitespace`
	_, _ = nc.SubscribeSync("foo.>.bar")                     // want `subject "foo.>.bar" is invalid: '>' must be the last token`
	_ = nc.Publish("", nil)                                  // want `subject "" is invalid: empty subject`
	_ = nc.PublishRequest("req", "reply..x", nil)            // want `subject "reply..x" is invalid: empty token`
	_ = nc.Publish(prefix+"..new", nil)                      // want `subject "orders..new" is invalid: empty token`
	_, _ = nc.Request(".x", nil, 0)                          // want `subject ".x" is invalid: empty token`
	_, _ = nc.RequestWithContext(context.TODO(), "a b", nil) // want `subject "a b" is invalid: contains whitespace`
	_, _ = nc.QueueSubscribe("x", "q", handler)
	_, _ = nc.QueueSubscribe("x.", "q", handler) // want `subject "x." is invalid: empty token`
	_, _ = nc.ChanSubscribe("x..y", ch)          // want `subject "x..y" is invalid: empty token`

	_ = nc.Publish("orders.*", nil)              // want `publish subject "orders.\*" contains a wildcard; wildcards only match in subscriptions`
	_, _ = nc.Request("orders.>", nil, 0)        // want `publish subject "orders.>" contains a wildcard; wildcards only match in subscriptions`
	_ = nc.PublishRequest("req", "reply.*", nil) // want `publish subject "reply.\*" contains a wildcard; wildcards only match in subscriptions`
	_ = nats.NewMsg("orders.*")                  // want `publish subject "orders.\*" contains a wildcard; wildcards only match in subscriptions`
	_, _ = nc.Subscribe("orders.*", handler)
	_, _ = nc.Subscribe("*.bar", handler)
	_, _ = nc.SubscribeSync("foo.>")
	_ = nc.Publish("foo*bar", nil)
	_ = nc.Publish("a.b>", nil)
	_ = nc.Publish("foo.bar", nil)

	_, _ = nc.QueueSubscribe("orders", "order workers", handler) // want `queue group "order workers" contains whitespace \(ErrBadQueueName\)`
	_, _ = nc.QueueSubscribeSync("orders", "q\t1")               // want `queue group "q\\t1" contains whitespace`
	_, _ = nc.ChanQueueSubscribe("orders", "a b", ch)            // want `queue group "a b" contains whitespace`
	_, _ = nc.QueueSubscribeSyncWithChan("orders", "a b", ch)    // want `queue group "a b" contains whitespace`
	_, _ = nc.QueueSubscribe("orders", "workers", handler)
	_, _ = nc.QueueSubscribe("orders", "", handler)

	_ = nc.Publish(fmt.Sprintf("orders.%s", id), nil)
	_ = nc.Publish(subj, nil)
}

var _ = []nats.Msg{
	{Subject: ""},         // want `subject "" is invalid: empty subject`
	{Subject: "orders.*"}, // want `publish subject "orders.\*" contains a wildcard; wildcards only match in subscriptions`
	{Subject: "orders.new", Reply: "inbox..1"}, // want `subject "inbox..1" is invalid: empty token`
	{Subject: "orders.new", Reply: "_INBOX.abc"},
}
