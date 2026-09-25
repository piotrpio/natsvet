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

package headerkey

import "testing"

func TestNearMiss(t *testing.T) {
	tests := []struct {
		key, want string
		ok        bool
	}{
		{"Nats-UpTo-Sequnce", "Nats-UpTo-Sequence", true},
		{"nats-msgid", "Nats-Msg-Id", true},
		{"Nats-Schedulex", "Nats-Schedule", true}, // distance 1 to Nats-Schedule and Nats-Scheduler: lexically first
		{"Nats-TTLSeconds", "Nats-TTL", true},
		{"Nats-Schedule-TTLSeconds", "Nats-Schedule-TTL", true},
		{"Nats-SchedulerXYZ12", "Nats-Scheduler", true}, // extends Nats-Schedule and Nats-Scheduler: longest
		{"Nats-TTL-Seconds", "", false},
		{"Nats-Has-More", "", false},
		{"Nats-X", "", false},
		{"Data-TTL", "", false},
	}
	for _, tt := range tests {
		got, ok := nearMiss(tt.key)
		if got != tt.want || ok != tt.ok {
			t.Errorf("nearMiss(%q) = %q, %v; want %q, %v", tt.key, got, ok, tt.want, tt.ok)
		}
	}
}
