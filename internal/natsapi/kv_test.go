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

package natsapi

import "testing"

func TestBucketValid(t *testing.T) {
	tests := []struct {
		bucket string
		want   bool
	}{
		{"users", true},
		{"user-profiles_v2", true},
		{"A1", true},
		{"", false},
		{"my.bucket", false},
		{"my bucket", false},
		{"files/2024", false},
		{"bücket", false},
	}
	for _, tt := range tests {
		if got := BucketValid(tt.bucket); got != tt.want {
			t.Errorf("BucketValid(%q) = %v, want %v", tt.bucket, got, tt.want)
		}
	}
}

func TestKeyValid(t *testing.T) {
	tests := []struct {
		key    string
		key_   bool
		search bool
	}{
		{"users", true, true},
		{"users/42=profile.v1", true, true},
		{"a.b.c", true, true},
		{"", false, false},
		{".hidden", false, false},
		{"trailing.", false, false},
		{"a..b", false, false},
		{"user name", false, false},
		{"users.*", false, true},
		{"users.>", false, true},
		{">", false, true},
		{"*", false, true},
		{"users.>.x", false, false},
		{"users.*.>", false, true},
		{"a.>b", false, false},
	}
	for _, tt := range tests {
		if got := KeyValid(tt.key); got != tt.key_ {
			t.Errorf("KeyValid(%q) = %v, want %v", tt.key, got, tt.key_)
		}
		if got := SearchKeyValid(tt.key); got != tt.search {
			t.Errorf("SearchKeyValid(%q) = %v, want %v", tt.key, got, tt.search)
		}
	}
}
