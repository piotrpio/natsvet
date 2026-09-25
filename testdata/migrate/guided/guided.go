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

// Package guided has a component with a mechanical site and a guided one,
// which an agent migrates by hand between plans.
package guided

import "github.com/nats-io/nats.go"

func Drain(nc *nats.Conn) (int, error) {
	js, err := nc.JetStream()
	if err != nil {
		return 0, err
	}
	info, err := js.AccountInfo()
	if err != nil {
		return 0, err
	}
	acked := 0
	// guided: begin
	sub, err := js.PullSubscribe("orders.new", "worker", nats.BindStream("ORDERS"))
	if err != nil {
		return 0, err
	}
	msgs, err := sub.Fetch(10)
	if err != nil {
		return 0, err
	}
	for _, m := range msgs {
		if err := m.Ack(); err != nil {
			return 0, err
		}
		acked++
	}
	// guided: end
	return info.Streams + acked, nil
}
