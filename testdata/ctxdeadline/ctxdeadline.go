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
package ctxdeadline

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const flushMsg = "FlushWithContext requires a context with a deadline"

func core(nc *nats.Conn, sub *nats.Subscription, ctx context.Context, msg *nats.Msg) {
	_ = nc.FlushWithContext(context.Background())                // want `FlushWithContext requires a context with a deadline \(returns ErrNoDeadlineContext\); use context.WithTimeout`
	_ = nc.FlushWithContext(context.TODO())                      // want `FlushWithContext requires a context with a deadline`
	_, _ = nc.RequestWithContext(context.Background(), "s", nil) // want `RequestWithContext with a context that has no deadline blocks forever if a responder exists but never replies; use context.WithTimeout`
	_, _ = nc.RequestMsgWithContext(context.TODO(), msg)         // want `RequestMsgWithContext with a context that has no deadline blocks forever`
	_, _ = sub.NextMsgWithContext(context.TODO())                // want `NextMsgWithContext with a context that has no deadline blocks forever`

	_ = nc.FlushWithContext(ctx)
	_, _ = nc.RequestWithContext(ctx, "s", nil)
	_, _ = nc.RequestWithContext(context.WithValue(context.Background(), "k", "v"), "s", nil)
	tctx, cancel := context.WithTimeout(context.Background(), 0)
	defer cancel()
	_ = nc.FlushWithContext(tctx)
}

func legacy(js nats.JetStreamContext, sub *nats.Subscription) {
	_, _ = sub.Fetch(10, nats.Context(context.Background())) // want `Fetch with nats.Context\(context.Background\(\)\) returns ErrNoDeadlineContext; use nats.MaxWait or a context with a deadline`
	_, _ = sub.FetchBatch(10, nats.Context(context.TODO()))  // want `FetchBatch with nats.Context\(context.TODO\(\)\) returns ErrNoDeadlineContext`
	_, _ = sub.Fetch(10, nats.MaxWait(0))
	_, _ = js.Subscribe("s", nil, nats.Context(context.Background()))
}

func newAPI(js jetstream.JetStream, c jetstream.Consumer) {
	_, _ = js.Publish(context.Background(), "s", nil)
	_, _ = c.Fetch(1)
	_, _ = c.Next()
}
