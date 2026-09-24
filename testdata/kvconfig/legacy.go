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

package kvconfig

import "github.com/nats-io/nats.go"

var _ = nats.KeyValueConfig{Bucket: "bad bucket", History: 65} // want `invalid bucket name "bad bucket"` `KV history 65 exceeds the maximum of 64`

var _ = nats.ObjectStoreConfig{Bucket: "ok_bucket"}

var _ = &nats.KeyValueConfig{Bucket: "orders", RePublish: &nats.RePublish{Source: "$KV.orders.>", Destination: "$KV.orders.copy.>"}} // want `republish destination "\$KV\.orders\.copy\.>" forms a cycle with the bucket subject "\$KV\.orders\.>"`

func legacy(js nats.JetStreamContext, kv nats.KeyValue) {
	_, _ = js.KeyValue("a.b")           // want `invalid bucket name "a.b"`
	_, _ = js.ObjectStore("a b")        // want `invalid bucket name "a b"`
	_, _ = kv.PutString("bad key", "v") // want `invalid KV key "bad key"`
	_, _ = kv.Get(".x")                 // want `invalid KV key ".x"`
	_, _ = kv.Watch("a.>.b")            // want `invalid KV key filter "a.>.b"`
	_, _ = kv.Put("good.key", nil)
	_, _ = kv.Watch("good.*")
}
