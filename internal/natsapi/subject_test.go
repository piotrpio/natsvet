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

// The tables below are nats-server's own (server/sublist_test.go:
// TestSublistValidSubjects, TestSubjectIsLiteral, TestIsSubsetMatch,
// TestSublistSubjectCollide) so the port is checked against the server's
// semantics rather than our reading of them.

func TestIsValidSubject(t *testing.T) {
	tests := []struct {
		subject string
		want    bool
	}{
		{".", false},
		{".foo", false},
		{"foo.", false},
		{"foo..bar", false},
		{">.bar", false},
		{"foo.>.bar", false},
		{"foo", true},
		{"foo.bar.*", true},
		{"foo.bar.>", true},
		{"*", true},
		{">", true},
		{"foo*", true},
		{"foo**", true},
		{"foo.**", true},
		{"foo*bar", true},
		{"foo.*bar", true},
		{"foo*.bar", true},
		{"*bar", true},
		{"foo>", true},
		{"foo.>>", true},
		{"foo>bar", true},
		{"foo.>bar", true},
		{"foo>.bar", true},
		{">bar", true},
		{"", false},
		{"foo bar", false},
		{"foo.\tbar", false},
		{"foo.bar.baz.\x00", false},
		{"foo.\xff", false},
	}
	for _, tt := range tests {
		if got := IsValidSubject(tt.subject); got != tt.want {
			t.Errorf("IsValidSubject(%q) = %v, want %v", tt.subject, got, tt.want)
		}
	}
}

func TestSubjectIsLiteral(t *testing.T) {
	tests := []struct {
		subject string
		want    bool
	}{
		{"foo", true},
		{"foo.bar", true},
		{"foo*.bar", true},
		{"*", false},
		{">", false},
		{"foo.*", false},
		{"foo.>", false},
		{"foo.*.>", false},
		{"foo.*.bar", false},
		{"foo.bar.>", false},
	}
	for _, tt := range tests {
		if got := SubjectIsLiteral(tt.subject); got != tt.want {
			t.Errorf("SubjectIsLiteral(%q) = %v, want %v", tt.subject, got, tt.want)
		}
	}
}

func TestSubjectIsSubsetMatch(t *testing.T) {
	tests := []struct {
		subject string
		test    string
		want    bool
	}{
		{"foo.bar", "foo.bar", true},
		{"foo.*", ">", true},
		{"foo.*", "*.*", true},
		{"foo.*", "foo.*", true},
		{"foo.*", "foo.bar", false},
		{"foo.>", ">", true},
		{"foo.>", "*.>", true},
		{"foo.>", "foo.>", true},
		{"foo.>", "foo.bar", false},
		{"foo..bar", "foo.*", false},
		{"foo.*", "foo..bar", false},
	}
	for _, tt := range tests {
		if got := SubjectIsSubsetMatch(tt.subject, tt.test); got != tt.want {
			t.Errorf("SubjectIsSubsetMatch(%q, %q) = %v, want %v", tt.subject, tt.test, got, tt.want)
		}
	}
}

func TestSubjectsCollide(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"foo.*", "foo.*.bar.>", false},
		{"foo.*.bar.>", "foo.*", false},
		{"foo.*", "foo.foo", true},
		{"foo.*", "*.foo", true},
		{"foo.bar.>", "*.bar.foo", true},
		{"orders.>", "orders.new", true},
		{"a.*.c", "a.b.>", true},
		{"orders.new", "orders.paid", false},
		{"orders.new", "orders.new", true},
	}
	for _, tt := range tests {
		if got := SubjectsCollide(tt.a, tt.b); got != tt.want {
			t.Errorf("SubjectsCollide(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
