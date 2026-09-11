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

package streamconfig

import (
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

var (
	name   string
	prefix string
)

const desc4096 = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

var _ = []jetstream.StreamConfig{
	// Constant-only policy.
	{Name: name, Replicas: 7}, // want `stream config: maximum replicas is 5`

	// Name.
	{Name: "orders.v1"}, // want `stream config: stream name is required and can not contain`
	{Name: ""},          // want `stream config: stream name is required and can not contain`
	{Subjects: []string{"a"}},
	{Name: "ORDERS_v1-a"},
	{Name: desc4096}, // want `stream config: stream name is too long, maximum allowed is 255`

	// Description.
	{Name: "s", Description: desc4096 + "x"}, // want `stream config: stream description is too long, maximum allowed is 4096`
	{Name: "s", Description: desc4096},

	// Replicas.
	{Name: "s", Replicas: 7},  // want `stream config: maximum replicas is 5`
	{Name: "s", Replicas: -1}, // want `stream config: replicas count cannot be negative`
	{Name: "s", Replicas: 3},
	{Name: "s"},

	// MaxAge and Duplicates.
	{Name: "s", MaxAge: 50 * time.Millisecond},              // want `stream config: max age needs to be >= 100ms`
	{Name: "s", MaxAge: -time.Second},                       // want `stream config: max age can not be negative`
	{Name: "s", MaxAge: time.Minute, Duplicates: time.Hour}, // want `stream config: duplicates window can not be larger then max age`
	{Name: "s", Duplicates: -time.Second},                   // want `stream config: duplicates window can not be negative`
	{Name: "s", Duplicates: 10 * time.Millisecond},          // want `stream config: duplicates window needs to be >= 100ms`
	{Name: "s", MaxAge: time.Minute},
	{Name: "s", MaxAge: 24 * time.Hour, Duplicates: 2 * time.Minute},

	// Rollup.
	{Name: "s", DenyPurge: true, AllowRollup: true}, // want `stream config: roll-ups require the purge permission`
	{Name: "s", AllowRollup: true},

	// Counters.
	{Name: "s", AllowMsgCounter: true, Retention: jetstream.WorkQueuePolicy}, // want `stream config: counter stream can only use limits retention`
	{Name: "s", AllowMsgCounter: true, Discard: jetstream.DiscardNew},        // want `stream config: counter stream cannot use discard new`
	{Name: "s", AllowMsgCounter: true, AllowMsgTTL: true},                    // want `stream config: counter stream cannot use message TTLs`
	{Name: "s", AllowMsgCounter: true, AllowMsgSchedules: true},              // want `stream config: counter stream cannot use message schedules`
	{Name: "s", AllowMsgCounter: true},

	// Discard new per subject.
	{Name: "s", DiscardNewPerSubject: true, MaxMsgsPerSubject: 10},         // want `stream config: discard new per subject requires discard new policy to be set`
	{Name: "s", DiscardNewPerSubject: true, Discard: jetstream.DiscardNew}, // want `stream config: discard new per subject requires max msgs per subject > 0`
	{Name: "s", DiscardNewPerSubject: true, Discard: jetstream.DiscardNew, MaxMsgsPerSubject: 10},

	// Subject delete marker TTL.
	{Name: "s", SubjectDeleteMarkerTTL: 500 * time.Millisecond}, // want `stream config: subject delete marker TTL must be at least 1 second`
	{Name: "s", SubjectDeleteMarkerTTL: -1},                     // want `stream config: subject delete marker TTL must not be negative`
	{Name: "s", SubjectDeleteMarkerTTL: time.Minute, AllowMsgTTL: true},

	// Scheduling.
	{Name: "s", AllowMsgSchedules: true, Discard: jetstream.DiscardNew},                   // want `stream config: message scheduling cannot use discard new`
	{Name: "s", AllowMsgSchedules: true, Sources: []*jetstream.StreamSource{{Name: "A"}}}, // want `stream config: stream source can not also schedule messages`
	{Name: "s", AllowMsgSchedules: true, AllowRollup: true},

	// Async persist.
	{Name: "s", PersistMode: jetstream.AsyncPersistMode, Replicas: 3},                      // want `stream config: async persist mode is not supported on replicated streams`
	{Name: "s", PersistMode: jetstream.AsyncPersistMode, Storage: jetstream.MemoryStorage}, // want `stream config: async persist mode is only supported on file storage`
	{Name: "s", PersistMode: jetstream.AsyncPersistMode, AllowAtomicPublish: true},         // want `stream config: async persist mode is not supported with atomic batch publish`
	{Name: "s", PersistMode: jetstream.AsyncPersistMode},

	// Mirror.
	{Name: "M", Mirror: &jetstream.StreamSource{Name: "ORDERS"}, Subjects: []string{"orders.>"}},             // want `stream config: stream mirrors can not contain subjects`
	{Name: "M", Mirror: &jetstream.StreamSource{Name: "A"}, Sources: []*jetstream.StreamSource{{Name: "B"}}}, // want `stream config: stream mirrors can not also contain other sources`
	{Name: "M", Mirror: &jetstream.StreamSource{Name: "A"}, FirstSeq: 10},                                    // want `stream config: stream mirrors can not have first sequence configured`
	{Name: "M", Mirror: &jetstream.StreamSource{Name: "A"}, AllowMsgCounter: true},                           // want `stream config: stream mirrors can not also calculate counters`
	{Name: "M", Mirror: &jetstream.StreamSource{Name: "A"}, AllowAtomicPublish: true},                        // want `stream config: stream mirrors can not also use atomic publishing`
	{Name: "M", Mirror: &jetstream.StreamSource{Name: "A"}, AllowBatchPublish: true},                         // want `stream config: stream mirrors can not also use batch publishing`
	{Name: "M", Mirror: &jetstream.StreamSource{Name: "A"}, AllowMsgSchedules: true},                         // want `stream config: stream mirrors can not also schedule messages`
	{Name: "M", Mirror: &jetstream.StreamSource{Name: "A"}, SubjectDeleteMarkerTTL: time.Minute},             // want `stream config: subject delete markers forbidden on mirrors`
	{Name: "M", Mirror: &jetstream.StreamSource{Name: "ORDERS"}},
	{Name: "M", Mirror: nil, Subjects: []string{"orders.>"}},

	// Subjects.
	{Name: "s", Subjects: []string{"orders.>", "orders.new"}},      // want `stream config: subject "orders.>" overlaps with "orders.new"`
	{Name: "s", Subjects: []string{"a", "a"}},                      // want `stream config: duplicate subjects detected`
	{Name: "s", Subjects: []string{"orders. new"}},                 // want `stream config: invalid subject "orders. new"`
	{Name: "s", Subjects: []string{">"}},                           // want `stream config: capturing all subjects requires no-ack to be true`
	{Name: "s", Subjects: []string{">"}, NoAck: true, Replicas: 3}, // want `stream config: capturing all subjects requires replicas of 1`
	{Name: "s", Subjects: []string{">"}, NoAck: true},
	{Name: "s", Subjects: []string{"$JS.API.>"}},     // want `stream config: subjects that overlap with jetstream api require no-ack to be true`
	{Name: "s", Subjects: []string{"$SYS.SERVER.>"}}, // want `stream config: subjects that overlap with system api require no-ack to be true`
	{Name: "s", Subjects: []string{"$JS.EVENT.ADVISORY.>"}},
	{Name: "s", Subjects: []string{"$SYS.ACCOUNT.>"}},
	{Name: "s", Subjects: []string{"$JS.API.>"}, NoAck: true},
	{Name: "s", Subjects: []string{"orders.new", "orders.paid", "shipments.*"}},
	{Name: "s", Subjects: []string{"a", "a", prefix + ".x"}}, // want `stream config: duplicate subjects detected`
	{Name: "s", Subjects: nil},
}

var _ = &jetstream.StreamConfig{Name: "a.b"} // want `stream config: stream name is required and can not contain`

type other struct {
	Name     string
	Replicas int
}

var _ = other{Name: "a.b", Replicas: 9}

func completedLater() {
	cfg := jetstream.StreamConfig{Name: "s", DiscardNewPerSubject: true, MaxMsgsPerSubject: 10}
	cfg.Discard = jetstream.DiscardNew
	_ = cfg
	m := jetstream.StreamConfig{Name: "M", Mirror: &jetstream.StreamSource{Name: "A"}, Subjects: []string{"a"}}
	m.Subjects = nil
	_ = m
	bad := jetstream.StreamConfig{Name: "s", Replicas: 7} // want `stream config: maximum replicas is 5`
	bad.Subjects = []string{"x"}
	_ = bad
}
