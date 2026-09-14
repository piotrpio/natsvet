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

import "github.com/nats-io/nats.go"

func legacy(js nats.JetStreamContext) {
	_, _ = js.Publish("orders.*", nil) // want `publish subject "orders.\*" contains a wildcard; wildcards only match in subscriptions`
	_, _ = js.SubscribeSync("orders.>")
	_, _ = js.Subscribe("orders..x", handler)          // want `subject "orders..x" is invalid: empty token`
	_, _ = js.QueueSubscribe("orders", "a b", handler) // want `queue group "a b" contains whitespace`
	_, _ = js.PullSubscribe("orders.>.x", "d")         // want `subject "orders.>.x" is invalid: '>' must be the last token`
	_, _ = js.QueueSubscribe("orders", "", handler)
	_, _ = js.SubscribeSync("", nats.BindStream("ORDERS"))
	_, _ = js.PullSubscribe("", "d", nats.Bind("ORDERS", "d"))
	_, _ = js.Publish("", nil) // want `subject "" is invalid: empty subject`
}
