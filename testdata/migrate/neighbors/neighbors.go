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

// Package neighbors has two subscribe calls on consecutive lines, answered
// differently, and a component of legacy-API code the user keeps.
package neighbors

import (
	"errors"

	"github.com/nats-io/nats.go"
)

func Handle(*nats.Msg) {}

func Subscribe(js nats.JetStreamContext) error {
	_, errNew := js.Subscribe("orders.new", Handle, nats.Durable("new"), nats.ManualAck())
	_, errOld := js.Subscribe("orders.old", Handle, nats.Durable("old"), nats.ManualAck())
	return errors.Join(errNew, errOld)
}
