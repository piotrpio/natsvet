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
	"context"
	"log"
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

var (
	subj     string
	t        time.Time
	longDesc = "x"
)

const desc4096 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

var _ = []jetstream.ConsumerConfig{
	// Constant-only policy.
	{FilterSubject: subj, FilterSubjects: []string{"a"}},
	{Durable: "orders.worker"}, // want `consumer config: consumer durable name can not contain`
	{Name: "my worker"},        // want `consumer config: consumer name can not contain`
	{Name: "orders-worker_1", Durable: ""},

	// Negatives.
	{AckWait: -time.Second},                     // want `consumer config: consumer ack wait needs to be positive`
	{BackOff: []time.Duration{time.Second, -1}}, // want `consumer config: consumer backoff needs to be positive`
	{Replicas: -1},                              // want `consumer config: replicas count cannot be negative`
	{Replicas: 0, AckWait: 0, BackOff: []time.Duration{time.Second}},

	// BackOff vs MaxDeliver.
	{MaxDeliver: 2, BackOff: []time.Duration{1, 2, 3}}, // want `consumer config: max deliver is required to be > length of backoff values`
	{BackOff: []time.Duration{1, 2, 3}},
	{MaxDeliver: -1, BackOff: []time.Duration{1, 2, 3}},
	{MaxDeliver: 3, BackOff: []time.Duration{1, 2, 3}},

	// Description.
	{Description: desc4096 + "x"}, // want `consumer config: consumer description is too long, maximum allowed is 4096`
	{Description: desc4096},
	{Description: longDesc},

	// Push-only.
	{DeliverSubject: "deliver.*"},                                                 // want `consumer config: consumer deliver subject has wildcards`
	{DeliverSubject: "deliver..x"},                                                // want `consumer config: invalid push consumer deliver subject`
	{DeliverSubject: "deliver.x", MaxWaiting: 10},                                 // want `consumer config: consumer in push mode can not set max waiting`
	{DeliverSubject: "d", AckPolicy: jetstream.AckNonePolicy, MaxAckPending: 100}, // want `consumer config: consumer requires ack policy for max ack pending`
	{AckPolicy: jetstream.AckNonePolicy, MaxAckPending: 100},
	{DeliverSubject: "d", IdleHeartbeat: 50 * time.Millisecond}, // want `consumer config: consumer idle heartbeat needs to be >= 100ms`
	{DeliverSubject: "deliver.x", IdleHeartbeat: time.Second, FlowControl: true},

	// Pull-only.
	{Durable: "w", IdleHeartbeat: 5 * time.Second}, // want `consumer config: consumer idle heartbeat requires a push based consumer`
	{RateLimit: 1000},                           // want `consumer config: consumer in pull mode can not have rate limit set`
	{MaxWaiting: -1},                            // want `consumer config: consumer max waiting needs to be positive`
	{FlowControl: true},                         // want `consumer config: consumer flow control requires a push based consumer` `consumer config: consumer with flow control also needs heartbeats`
	{MaxRequestBatch: -1},                       // want `consumer config: consumer max request batch needs to be > 0`
	{MaxRequestExpires: 500 * time.Microsecond}, // want `consumer config: consumer max request expires needs to be >= 1ms`
	{Durable: "w", MaxWaiting: 512, MaxRequestBatch: 100, MaxRequestExpires: time.Second},
	{DeliverSubject: subj, IdleHeartbeat: 5 * time.Second, RateLimit: 1000},

	// Filters.
	{FilterSubject: "a", FilterSubjects: []string{"b"}},  // want `consumer config: consumer cannot have both FilterSubject and FilterSubjects specified`
	{FilterSubjects: []string{"orders.*", ""}},           // want `consumer config: consumer filter in FilterSubjects cannot be empty`
	{FilterSubjects: []string{"orders.*", "orders.new"}}, // want `consumer config: consumer subject filters cannot overlap`
	{FilterSubjects: []string{"orders.new", "orders.paid"}},
	{FilterSubject: "orders..new"}, // want `consumer config: invalid filter subject "orders..new"`
	{FilterSubjects: []string{"orders.new", subj}},

	// Deliver policy vs start options.
	{OptStartSeq: 10}, // want `consumer config: consumer delivery policy is deliver all, but optional start sequence is also set`
	{DeliverPolicy: jetstream.DeliverLastPolicy, OptStartTime: &t},                            // want `consumer config: consumer delivery policy is deliver last, but optional start time is also set`
	{DeliverPolicy: jetstream.DeliverByStartSequencePolicy},                                   // want `consumer config: consumer delivery policy is deliver by start sequence, but optional start sequence is not set`
	{DeliverPolicy: jetstream.DeliverByStartSequencePolicy, OptStartSeq: 5, OptStartTime: &t}, // want `consumer config: consumer delivery policy is deliver by start sequence, but optional start time is also set`
	{DeliverPolicy: jetstream.DeliverByStartTimePolicy},                                       // want `consumer config: consumer delivery policy is deliver by start time, but optional start time is not set`
	{DeliverPolicy: jetstream.DeliverByStartTimePolicy, OptStartTime: &t, OptStartSeq: 3},     // want `consumer config: consumer delivery policy is deliver by start time, but optional start start sequence is also set`
	{DeliverPolicy: jetstream.DeliverByStartTimePolicy, OptStartTime: &t},
	{DeliverPolicy: jetstream.DeliverByStartSequencePolicy, OptStartSeq: 5},
	{DeliverPolicy: jetstream.DeliverLastPerSubjectPolicy}, // want `consumer config: consumer delivery policy is deliver last per subject, but optional filter subject is not set`
	{DeliverPolicy: jetstream.DeliverLastPerSubjectPolicy, FilterSubject: "a"},
	{DeliverPolicy: jetstream.DeliverNewPolicy, OptStartTime: nil},

	// Sampling.
	{SampleFrequency: "half"}, // want `consumer config: failed to parse consumer sampling configuration`
	{SampleFrequency: "50%"},
	{SampleFrequency: "50"},

	// Flow control needs heartbeats.
	{DeliverSubject: "d", FlowControl: true}, // want `consumer config: consumer with flow control also needs heartbeats`
	{DeliverSubject: "d", FlowControl: true, IdleHeartbeat: time.Second},

	// Durable and Name.
	{Durable: "a", Name: "b"}, // want `consumer config: Consumer Durable and Name have to be equal if both are provided`
	{Durable: "a", Name: "a"},
	{Durable: "a"},

	// Priority.
	{PriorityPolicy: jetstream.PriorityPolicyOverflow},                                                      // want `consumer config: Setting PriorityPolicy requires at least one PriorityGroup to be set`
	{PriorityPolicy: jetstream.PriorityPolicyOverflow, PriorityGroups: []string{"a"}, DeliverSubject: "d"},  // want `consumer config: priority groups can not be used with push consumers`
	{PriorityPolicy: jetstream.PriorityPolicyPinned, PriorityGroups: []string{""}},                          // want `consumer config: Group name cannot be an empty string`
	{PriorityPolicy: jetstream.PriorityPolicyPinned, PriorityGroups: []string{"this-name-is-way-too-long"}}, // want `consumer config: Valid priority group name must match`
	{PriorityGroups: []string{"a"}}, // want `consumer config: consumer can not have priority groups when policy is none`
	{PinnedTTL: time.Minute},        // want `consumer config: PinnedTTL cannot be set when PriorityPolicy is none`
	{PriorityPolicy: jetstream.PriorityPolicyPinned, PriorityGroups: []string{"gold", "silver"}, PinnedTTL: time.Minute},

	// Flow-control ack policy.
	{AckPolicy: jetstream.AckFlowControlPolicy}, // want `consumer config: flow control ack policy requires a push based consumer` `consumer config: flow control ack policy requires flow control` `consumer config: flow control ack policy heartbeat needs to be 1s` `consumer config: flow control ack policy requires max ack pending`
	{AckPolicy: jetstream.AckFlowControlPolicy, DeliverSubject: "d", FlowControl: true, IdleHeartbeat: time.Second, MaxAckPending: 1000},
	{AckPolicy: jetstream.AckFlowControlPolicy, DeliverSubject: "d", FlowControl: true, IdleHeartbeat: time.Second, MaxAckPending: 1000, AckWait: time.Second, MaxDeliver: 3}, // want `consumer config: flow control ack policy requires unset ack wait` `consumer config: flow control ack policy requires unset max deliver`
}

var _ = &jetstream.ConsumerConfig{Name: "a.b"} // want `consumer config: consumer name can not contain`

type other struct {
	FilterSubject  string
	FilterSubjects []string
}

var _ = other{FilterSubject: "a", FilterSubjects: []string{"b"}}

func completedLater() {
	cfg := jetstream.ConsumerConfig{DeliverPolicy: jetstream.DeliverByStartSequencePolicy}
	cfg.OptStartSeq = 10
	_ = cfg
	prio := jetstream.ConsumerConfig{PriorityPolicy: jetstream.PriorityPolicyOverflow}
	prio.PriorityGroups = []string{"a"}
	_ = prio
	fixed := jetstream.ConsumerConfig{Durable: "a.b"} // want `consumer config: consumer durable name can not contain`
	fixed.FilterSubject = "x"
	_ = fixed
}

func templateConfig(js jetstream.JetStream) {
	cfg := jetstream.ConsumerConfig{Durable: "a", IdleHeartbeat: 5 * time.Second} // want `consumer config: consumer idle heartbeat requires a push based consumer`
	_, _ = js.CreateConsumer(ctx, "S", cfg)
	cfg.Durable = "b"
	cfg.DeliverSubject = "deliver.b"
	_, _ = js.CreateConsumer(ctx, "S", cfg)

	byWrapper := jetstream.ConsumerConfig{Durable: "w.x"} // want `consumer config: consumer durable name can not contain`
	createCons(byWrapper)
	byWrapper.Durable = "w"
	createCons(byWrapper)

	byPointer := jetstream.ConsumerConfig{IdleHeartbeat: 5 * time.Second}
	fillAndCreate(&byPointer)
	byPointer.DeliverSubject = "d"

	logged := jetstream.ConsumerConfig{IdleHeartbeat: 5 * time.Second}
	log.Printf("creating %v", logged)
	logged.DeliverSubject = "deliver.l"
	_, _ = js.CreateConsumer(ctx, "S", logged)

	_, _ = js.CreateConsumer(ctx, "S", jetstream.ConsumerConfig{Durable: "in.line"}) // want `consumer config: consumer durable name can not contain`
	var unrelated jetstream.StreamConfig
	unrelated.Name = "masks nothing for the inline literal above"
	_ = unrelated
}

func createCons(cfg jetstream.ConsumerConfig)     {}
func fillAndCreate(cfg *jetstream.ConsumerConfig) {}

var ctx = context.Background()
