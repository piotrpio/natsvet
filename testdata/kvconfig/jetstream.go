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

import (
	"context"

	"github.com/nats-io/nats.go/jetstream"
)

var (
	name string
	key  string
	v    []byte
	ctx  = context.Background()
)

var _ = []jetstream.KeyValueConfig{
	{Bucket: "my.bucket"}, // want `invalid bucket name "my.bucket": bucket names may only contain`
	{Bucket: "user-profiles_v2"},
	{Bucket: name},
	{Bucket: "b", History: 100}, // want `KV history 100 exceeds the maximum of 64`
	{Bucket: "b", History: 64},
	{Bucket: "b", History: 0},
	{Bucket: "b"},
}

var _ = &jetstream.ObjectStoreConfig{Bucket: "files/2024"} // want `invalid bucket name "files/2024"`

// Republish cycles against the bucket subject.
var _ = []jetstream.KeyValueConfig{
	{Bucket: "orders", RePublish: &jetstream.RePublish{Destination: ">"}}, // want `republish destination ">" forms a cycle with the bucket subject "\$KV\.orders\.>"; the server rejects the bucket`
	{Bucket: "orders", RePublish: &jetstream.RePublish{Source: "$KV.orders.>", Destination: "feed.orders.>"}},
	{Bucket: "orders", Mirror: &jetstream.StreamSource{Name: "orders"}, RePublish: &jetstream.RePublish{Destination: ">"}},
	{Bucket: name, RePublish: &jetstream.RePublish{Destination: ">"}},
	{Bucket: "orders", RePublish: &jetstream.RePublish{Source: "$KV.orders.>"}},
}

func lookups(js jetstream.JetStream) {
	_, _ = js.KeyValue(ctx, "my bucket") // want `invalid bucket name "my bucket"`
	_ = js.DeleteKeyValue(ctx, "a.b")    // want `invalid bucket name "a.b"`
	_, _ = js.ObjectStore(ctx, "x y")    // want `invalid bucket name "x y"`
	_ = js.DeleteObjectStore(ctx, "x/y") // want `invalid bucket name "x/y"`
	_, _ = js.KeyValue(ctx, "profiles")
	_, _ = js.KeyValue(ctx, name)
}

func keys(kv jetstream.KeyValue) {
	_, _ = kv.Put(ctx, "user name", v)                    // want `invalid KV key "user name": keys may only contain`
	_, _ = kv.Get(ctx, ".hidden")                         // want `invalid KV key ".hidden"`
	_ = kv.Delete(ctx, "a..b")                            // want `invalid KV key "a..b"`
	_, _ = kv.Put(ctx, "users.*", v)                      // want `invalid KV key "users.\*"`
	_, _ = kv.GetRevision(ctx, "trailing.", 1)            // want `invalid KV key "trailing."`
	_, _ = kv.PutString(ctx, "", "v")                     // want `invalid KV key ""`
	_, _ = kv.Create(ctx, "a b", v)                       // want `invalid KV key "a b"`
	_, _ = kv.Update(ctx, "a>b", v, 1)                    // want `invalid KV key "a>b"`
	_ = kv.Purge(ctx, "x..y")                             // want `invalid KV key "x..y"`
	_, _ = kv.Watch(ctx, "users.>.x")                     // want `invalid KV key filter "users.>.x": filters may only contain`
	_, _ = kv.History(ctx, ".x")                          // want `invalid KV key filter ".x"`
	_, _ = kv.WatchFiltered(ctx, []string{"ok.>", "a b"}) // want `invalid KV key filter "a b"`
	_, _ = kv.ListKeysFiltered(ctx, "ok.*", "bad..key")   // want `invalid KV key filter "bad..key"`

	_, _ = kv.Watch(ctx, "users.*")
	_, _ = kv.WatchFiltered(ctx, []string{"users.>", "orders.*"})
	_, _ = kv.Put(ctx, "users/42=profile.v1", v)
	_, _ = kv.Watch(ctx, ">")
	_, _ = kv.Put(ctx, key, v)
	_, _ = kv.WatchFiltered(ctx, []string{key})
}
