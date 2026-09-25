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

package objects

import (
	"context"

	"github.com/nats-io/nats.go"
)

func Store(ctx context.Context, nc *nats.Conn, data []byte) ([]byte, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}
	obj, err := js.ObjectStore("files")
	if err != nil {
		return nil, err
	}
	if _, err := obj.PutBytes("a", data); err != nil {
		return nil, err
	}
	return obj.GetBytes("a")
}
