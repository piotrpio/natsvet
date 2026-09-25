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

// Package order has sites whose facts come from several uses and several
// values, which the plan must list in source order.
package order

import "github.com/nats-io/nats.go"

type holder struct {
	sub *nats.Subscription
}

func consume(*nats.Subscription) {}

func Pull(js nats.JetStreamContext) (*holder, error) {
	sub, err := js.PullSubscribe("ORDERS.new", "worker")
	if err != nil {
		return nil, err
	}
	h := &holder{sub: sub}
	h.sub = sub
	consume(sub)
	return h, nil
}

func config() nats.StreamConfig { return nats.StreamConfig{} }

func Defaults() bool {
	tests := []struct {
		retention nats.RetentionPolicy
		storage   nats.StorageType
		discard   nats.DiscardPolicy
	}{
		{retention: nats.LimitsPolicy, storage: nats.FileStorage, discard: nats.DiscardOld},
	}
	got := config()
	for _, tt := range tests {
		if got.Retention != tt.retention || got.Storage != tt.storage || got.Discard != tt.discard {
			return false
		}
	}
	return true
}
