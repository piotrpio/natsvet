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

package syncsub

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
)

var (
	handler nats.MsgHandler
	ch      chan *nats.Msg
	ctx     context.Context
)

func callback(nc *nats.Conn) {
	sub, _ := nc.Subscribe("s", handler)
	_, _ = sub.NextMsg(time.Second) // want `NextMsg on a subscription created with Subscribe \(callback\) always returns ErrSyncSubRequired; use SubscribeSync`
	qsub, _ := nc.QueueSubscribe("s", "q", handler)
	_, _ = qsub.NextMsgWithContext(ctx) // want `NextMsg on a subscription created with QueueSubscribe \(callback\) always returns ErrSyncSubRequired; use QueueSubscribeSync`
}

func channels(nc *nats.Conn) {
	sub, _ := nc.ChanSubscribe("s", ch)
	_, _ = sub.NextMsg(time.Second) // want `NextMsg on a subscription created with ChanSubscribe steals messages from the channel; read the channel or use SubscribeSync`
	qsub, _ := nc.ChanQueueSubscribe("s", "q", ch)
	_, _ = qsub.NextMsg(time.Second) // want `NextMsg on a subscription created with ChanQueueSubscribe steals messages from the channel`
	wsub, _ := nc.QueueSubscribeSyncWithChan("s", "q", ch)
	_, _ = wsub.NextMsg(time.Second) // want `NextMsg on a subscription created with QueueSubscribeSyncWithChan steals messages from the channel`
}

func sync(nc *nats.Conn) {
	sub, _ := nc.SubscribeSync("s")
	_, _ = sub.NextMsg(time.Second)
	qsub, _ := nc.QueueSubscribeSync("s", "q")
	_, _ = qsub.NextMsg(time.Second)
}

func reassigned(nc *nats.Conn) {
	sub, _ := nc.Subscribe("s", handler)
	sub, _ = nc.SubscribeSync("s")
	_, _ = sub.NextMsg(time.Second)
}

func parameter(sub *nats.Subscription) {
	_, _ = sub.NextMsg(time.Second)
}

type holder struct{ sub *nats.Subscription }

func field(h *holder) {
	_, _ = h.sub.NextMsg(time.Second)
}
