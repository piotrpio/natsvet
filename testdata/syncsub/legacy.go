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
	"time"

	"github.com/nats-io/nats.go"
)

func legacy(js nats.JetStreamContext) {
	sub, _ := js.Subscribe("s", handler)
	_, _ = sub.NextMsg(time.Second) // want `NextMsg on a subscription created with Subscribe \(callback\) always returns ErrSyncSubRequired; use SubscribeSync`
	qsub, _ := js.QueueSubscribe("s", "q", handler)
	_, _ = qsub.NextMsgWithContext(ctx) // want `NextMsg on a subscription created with QueueSubscribe \(callback\)`
	csub, _ := js.ChanSubscribe("s", ch)
	_, _ = csub.NextMsg(time.Second) // want `NextMsg on a subscription created with ChanSubscribe steals messages from the channel`
	psub, _ := js.PullSubscribe("s", "d")
	_, _ = psub.NextMsg(time.Second) // want `NextMsg on a pull subscription returns ErrTypeSubscription; use Fetch`
	ssub, _ := js.SubscribeSync("s")
	_, _ = ssub.NextMsg(time.Second)
}
