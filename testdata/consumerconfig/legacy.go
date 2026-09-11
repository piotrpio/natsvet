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

package consumerconfig

import (
	"time"

	"github.com/nats-io/nats.go"
)

var _ = []nats.ConsumerConfig{
	{Durable: "a.b"},             // want `consumer config: consumer durable name can not contain`
	{Heartbeat: 5 * time.Second}, // want `consumer config: consumer idle heartbeat requires a push based consumer`
	{DeliverSubject: "d", AckPolicy: nats.AckNonePolicy, MaxAckPending: 100}, // want `consumer config: consumer requires ack policy for max ack pending`
	{DeliverPolicy: nats.DeliverByStartSequencePolicy},                       // want `consumer config: consumer delivery policy is deliver by start sequence, but optional start sequence is not set`
	{Durable: "w", AckPolicy: nats.AckExplicitPolicy, FilterSubject: "orders.>"},
}
