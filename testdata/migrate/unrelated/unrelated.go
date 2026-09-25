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

// Package unrelated already uses the jetstream package under a name a
// sibling would take, next to a function on the legacy API.
package unrelated

import (
	"context"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func Native(ctx context.Context, nc *nats.Conn) (int, error) {
	jsNew, err := jetstream.New(nc)
	if err != nil {
		return 0, err
	}
	info, err := jsNew.AccountInfo(ctx)
	if err != nil {
		return 0, err
	}
	return info.Streams, nil
}

func Legacy(nc *nats.Conn) (int, error) {
	js, err := nc.JetStream()
	if err != nil {
		return 0, err
	}
	info, err := js.AccountInfo()
	if err != nil {
		return 0, err
	}
	return info.Streams, nil
}
