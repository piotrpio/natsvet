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

	"github.com/nats-io/nats.go"
)

var _ = []nats.StreamConfig{
	{Name: "a.b"},            // want `stream config: stream name is required and can not contain`
	{Name: "s", Replicas: 6}, // want `stream config: maximum replicas is 5`
	{Name: "s", Subjects: []string{"a.>", "a.b"}},                 // want `stream config: subject "a.>" overlaps with "a.b"`
	{Name: "s", MaxAge: time.Minute, Duplicates: 2 * time.Minute}, // want `stream config: duplicates window can not be larger then max age`
	{Name: "s", Subjects: []string{"orders.>"}, Retention: nats.WorkQueuePolicy, Storage: nats.FileStorage, Replicas: 3},
}

var _ = &nats.StreamConfig{Name: "ok", Subjects: []string{"ok.>"}}
