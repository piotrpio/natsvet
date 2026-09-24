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

import (
	"errors"
	"fmt"
	"math"
	"testing"
)

// The tables below are nats-server's own (server/sublist_test.go:
// TestValidateDestinationSubject; server/subject_transform_test.go: the
// non-strict rows of TestSubjectTransforms, including the pairs its
// shouldMatch helper accepts) so the port is checked against the server's
// semantics rather than our reading of them.

func TestValidateMapping(t *testing.T) {
	tests := []struct {
		src, dest string
		want      error
	}{
		{"bar", "foo", nil},
		{"foo", "foo.bar", nil},
		{"*", "{{wildcard(1)}}", nil},
		{"foo.>", ">", nil},
		{"foo.*", "bar.{{wildcard(1)}}", nil},
		{"foo.>", "bar.>", nil},
		{"foo.*.>", "bar.{{wildcard(1)}}.>", nil},
		{"foo.*.bar", "bar.{{wildcard(1)}}.foo", nil},
		{"foo.bar.>", "foo.bar.foo.>", nil},
		{"*", "foo.{{wildcard(1)}}", nil},
		{"*", "foo.{{ wildcard(1) }}", nil},
		{"*", "foo.{{wildcard( 1 )}}", nil},
		{"*", "foo.{{partition(2,1)}}", nil},
		{"*.*", "foo.{{SplitFromLeft(2,1)}}", nil},
		{"*.*", "foo.{{SplitFromRight(2,1)}}", nil},
		{"*.*", "foo.{{SliceFromLeft(2,1)}}", nil},
		{"*.*", "foo.{{SliceFromRight(2,1)}}", nil},
		{"*.*", "foo.{{split(1,-)}}", nil},
		{"*", "foo.{{random(1)}}", nil},
		{"*", "foo.{{left(1,2)}}", nil},
		{"*", "foo.{{Left(1,2)}}", nil},
		{"*", "foo.{{right(1,2)}}", nil},
		{"*", "foo.{{Right(1,2)}}", nil},
		{"*", "foo.{{ left( 1 , 2 ) }}", nil},
		{"*", "foo.{{unknown(1)}}", errInvalidMappingDestination},
		{"foo", "foo..}", errInvalidMappingDestination},
		{"foo", "foo. bar}", errInvalidMappingDestinationSubject},
		{"orders.>", "", nil},
	}
	for _, tt := range tests {
		err := ValidateMapping(tt.src, tt.dest)
		switch {
		case tt.want == nil && err != nil:
			t.Errorf("ValidateMapping(%q, %q) = %v, want nil", tt.src, tt.dest, err)
		case tt.want != nil && !errors.Is(err, tt.want):
			t.Errorf("ValidateMapping(%q, %q) = %v, want %v", tt.src, tt.dest, err, tt.want)
		}
	}
}

func TestSubjectTransformErr(t *testing.T) {
	bad := []struct{ src, dest string }{
		{"foo..", "bar"},
		{"foo.*", "bar.*"},
		{"foo.*", "bar.$2"},
		{"foo.*", "bar.$1.>"},
		{"foo.>", "bar.baz"},
		{"foo.*", "foo.{{wildcard(2)}}"},
		{"foo.*", "foo.{{unimplemented(1)}}"},
		{"foo.*", "foo.{{partition()}}"},
		{"foo.*", "foo.{{random()}}"},
		{"foo.*", "foo.{{wildcard(foo)}}"},
		{"foo.*", "foo.{{wildcard()}}"},
		{"foo.*", "foo.{{wildcard(1,2)}}"},
		{"foo.*", "foo.{{ wildcard5) }}"},
		{"foo.*", "foo.{{splitLeft(2,2}}"},
		{"foo", "bla.{{wildcard(1)}}"},
		{"foo.*", fmt.Sprintf("foo.{{partition(%d)}}", math.MaxInt32+1)},
		{"foo.*", fmt.Sprintf("foo.{{random(%d)}}", math.MaxInt32+1)},
	}
	for _, tt := range bad {
		err := SubjectTransformErr(tt.src, tt.dest)
		if err != errBadSubject && !errors.Is(err, errInvalidMappingDestination) {
			t.Errorf("SubjectTransformErr(%q, %q) = %v, want a bad subject or mapping error", tt.src, tt.dest, err)
		}
	}
	ok := []struct{ src, dest string }{
		{"foo.*", "bar.{{Wildcard(1)}}"},
		{"foo.*.*", "bar.$2"},
		{"foo.*.*", "bar.{{wildcard(1)}}"},
		{"foo.*.*", "bar.{{partition(1)}}"},
		{"foo.*.*", "bar.{{random(5)}}"},
		{"foo", "bar"},
		{"foo.*.bar.*.baz", "req.$2.$1"},
		{"baz.>", "mybaz.>"},
		{"*", "{{splitfromleft(1,1)}}"},
		{"", "prefix.>"},
		{"*.*", "{{partition(10,1,2)}}"},
		{"foo.*.*", "foo.{{wildcard(1)}}.{{wildcard(2)}}.{{partition(5,1,2)}}"},
		{"foo.*", fmt.Sprintf("foo.{{partition(%d)}}", math.MaxInt32)},
		{"foo.*", fmt.Sprintf("foo.{{random(%d)}}", math.MaxInt32)},
		{"foo.bar", fmt.Sprintf("foo.{{random(%d)}}", math.MaxInt32)},
		{"foo", ""},
		{"foo.*.bar.*.baz", "req.{{wildcard(2)}}.{{wildcard(1)}}"},
		{"baz.>", "my.pre.>"},
		{"baz.>", "foo.bar.>"},
		{"*", "foo.bar.$1"},
		{"*", "{{splitfromleft(1,3)}}"},
		{"*", "{{SplitFromRight(1,3)}}"},
		{"*", "{{SliceFromLeft(1,3)}}"},
		{"*", "{{SliceFromRight(1,3)}}"},
		{"*", "{{split(1,-)}}"},
		{"*.*", "{{split(2,-)}}.{{splitfromleft(1,2)}}"},
		{"*", "{{right(1,1)}}"},
		{"*", "{{right(1,3)}}"},
		{"*", "{{right(1,6)}}"},
		{"*", "{{left(1,1)}}"},
		{"*", "{{left(1,3)}}"},
		{"*", "{{left(1,6)}}"},
		{"*", "bar.{{partition(0)}}"},
		{"*", "bar.{{partition(10, 0)}}"},
		{"*.*", "bar.{{partition(10)}}"},
		{"*", "bar.{{partition(10)}}"},
		{"*", "bar.{{random(0)}}"},
		{"*", "bar.{{random(6)}}"},
		{"foo.bar", "baz.{{partition(10)}}"},
		{"foo.baz", "qux.{{partition(10)}}"},
		{"test.subject", "result.{{partition(5)}}"},
	}
	for _, tt := range ok {
		if err := SubjectTransformErr(tt.src, tt.dest); err != nil {
			t.Errorf("SubjectTransformErr(%q, %q) = %v, want nil", tt.src, tt.dest, err)
		}
	}
}

func TestTransformErrorText(t *testing.T) {
	tests := []struct {
		validate  func(src, dest string) error
		src, dest string
		want      string
	}{
		{ValidateMapping, "events.*", "events.{{split(3,1)}}", "invalid mapping destination: wildcard index out of range in {{split(3,1)}}: [3]"},
		{ValidateMapping, "events.*.*", "events.{{wildcard(1)}}{{split(3,1)}}", "invalid mapping destination: too many arguments passed to the function in {{wildcard(1)}}{{split(3,1)}}"},
		{ValidateMapping, "a.*", "b.{{unknown(1)}}", "invalid mapping destination: unknown function in {{unknown(1)}}"},
		{ValidateMapping, "", "archive.orders", "invalid subject"},
		{ValidateMapping, "foo", "foo..}", "invalid mapping destination: invalid transform"},
		{SubjectTransformErr, "orders.*", "repub.$2", "invalid mapping destination: wildcard index out of range in $2: [2]"},
		{SubjectTransformErr, ">", "repub.orders", "invalid subject"},
	}
	for _, tt := range tests {
		err := tt.validate(tt.src, tt.dest)
		if err == nil || err.Error() != tt.want {
			t.Errorf("(%q, %q) = %v, want %q", tt.src, tt.dest, err, tt.want)
		}
	}
}
