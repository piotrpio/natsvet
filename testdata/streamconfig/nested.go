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
	"context"

	"github.com/nats-io/nats.go/jetstream"
)

// Subject transform sources and mappings.
var _ = []jetstream.SubjectTransformConfig{
	{Source: "events.>.*", Destination: "x.>"}, // want `stream config: subject transform source: invalid subject "events\.>\.\*"`
	{Destination: "x.>"},
	{Source: "events.*", Destination: "x.{{wildcard(1)}}"},
	{Source: "events.*", Destination: "events.{{split(3,1)}}"},                  // want `stream config: subject transform from "events\.\*" to "events\.{{split\(3,1\)}}": invalid mapping destination: wildcard index out of range in {{split\(3,1\)}}: \[3\]`
	{Source: "events.*.*", Destination: "events.{{wildcard(1)}}{{split(3,1)}}"}, // want `invalid mapping destination: too many arguments passed to the function in {{wildcard\(1\)}}{{split\(3,1\)}}$`
	{Source: "a.*", Destination: "b.{{unknown(1)}}"},                            // want `invalid mapping destination: unknown function in {{unknown\(1\)}}$`
	{Destination: "archive.orders"},                                             // want `stream config: subject transform from "" to "archive\.orders": invalid subject$`
	{Source: "orders.*.*", Destination: "archive.{{wildcard(2)}}.{{partition(3,1)}}"},
	{Source: "orders.>"},
	{Source: "events.>.*", Destination: "x.$1"}, // want `stream config: subject transform source: invalid subject "events\.>\.\*"`
}

// Republish mappings.
var _ = []jetstream.RePublish{
	{Source: "orders.*", Destination: "repub.$2"}, // want `stream config: republish with transform from "orders\.\*" to "repub\.\$2" not valid`
	{Destination: "repub.orders"},                 // want `stream config: republish with transform from ">" to "repub\.orders" not valid`
	{Source: "orders.>", Destination: "repub.orders.>"},
}

// Stream sources on their own.
var _ = []*jetstream.StreamSource{
	{Name: "A", FilterSubject: "a.>", SubjectTransforms: []jetstream.SubjectTransformConfig{{Source: "a.b", Destination: "c.b"}}}, // want `stream config: a source or mirror with subject transforms cannot also have a single subject filter`
	{Name: "A", FilterSubject: "a.>"},
	{Name: "A", FilterSubject: "", SubjectTransforms: []jetstream.SubjectTransformConfig{{Source: "a.b", Destination: "c.b"}}},
	{Name: "A", SubjectTransforms: []jetstream.SubjectTransformConfig{{Source: "orders.>", Destination: "a.>"}, {Source: "returns.>", Destination: "b.>"}}},
	{Name: "A", Domain: "hub", External: &jetstream.ExternalStream{APIPrefix: "$JS.hub.API"}}, // want `stream config: domain and external are both set`
	{Name: "A", Domain: "hub"},
	{Name: "O", Consumer: &jetstream.StreamConsumerSource{Name: "C"}}, // want `stream config: stream source consumer config is invalid: deliver subject must be a valid literal subject`
	{Name: "O", FilterSubject: "o.>", Consumer: &jetstream.StreamConsumerSource{Name: "C", DeliverSubject: "deliver.c"}}, // want `stream config: stream source consumer config is invalid: a filter subject can not be set`
	{Name: "O", OptStartSeq: 5, Consumer: &jetstream.StreamConsumerSource{Name: "C", DeliverSubject: "deliver.c"}},       // want `stream config: stream source consumer config is invalid: a start sequence or start time can not be set`
	{Name: "O", Consumer: &jetstream.StreamConsumerSource{Name: "c.1", DeliverSubject: "deliver.c"}},                     // want `stream config: stream source consumer config is invalid: consumer name is required and can not contain`
	{Name: "O", Consumer: &jetstream.StreamConsumerSource{Name: "C", DeliverSubject: "deliver.*"}},                       // want `stream config: stream source consumer config is invalid: deliver subject must be a valid literal subject`
	{Name: "O", Consumer: &jetstream.StreamConsumerSource{Name: "C", DeliverSubject: "deliver.c"}},
	{Name: "O", OptStartSeq: 10},
}

var _ = []jetstream.StreamConfig{
	// Overlapping transform sources: subset in a source, collision only in a mirror.
	{Name: "s", Sources: []*jetstream.StreamSource{{Name: "A", SubjectTransforms: []jetstream.SubjectTransformConfig{{Source: "orders.>", Destination: "a.>"}, {Source: "orders.new", Destination: "b"}}}}},     // want `stream config: subject transform sources "orders\.>" and "orders\.new" can not overlap`
	{Name: "M", Mirror: &jetstream.StreamSource{Name: "A", SubjectTransforms: []jetstream.SubjectTransformConfig{{Source: "orders.*.new", Destination: "x.$1"}, {Source: "orders.eu.*", Destination: "y.$1"}}}}, // want `stream config: subject transform sources "orders\.\*\.new" and "orders\.eu\.\*" can not overlap`
	{Name: "s", Sources: []*jetstream.StreamSource{{Name: "A", SubjectTransforms: []jetstream.SubjectTransformConfig{{Source: "orders.*.new", Destination: "x.$1"}, {Source: "orders.eu.*", Destination: "y.$1"}}}}},

	// Sourced and mirrored stream names.
	{Name: "s", Sources: []*jetstream.StreamSource{{Name: "orders.v1"}}}, // want `stream config: sourced stream name is invalid`
	{Name: "s", Sources: []*jetstream.StreamSource{nil}},                 // want `stream config: sourced stream name is invalid`
	{Name: "M", Mirror: &jetstream.StreamSource{}},                       // want `stream config: mirrored stream name is invalid`
	{Name: "M", Mirror: &jetstream.StreamSource{External: &jetstream.ExternalStream{APIPrefix: "$JS.hub.API"}}},
	{Name: "M", Mirror: &jetstream.StreamSource{Domain: "hub"}},

	// Republish cycles.
	{Name: "s", Subjects: []string{"orders.>"}, RePublish: &jetstream.RePublish{Source: "orders.>", Destination: "orders.copy.>"}}, // want `stream config: republish destination "orders\.copy\.>" forms a cycle with subject "orders\.>"`
	{Name: "ORDERS", RePublish: &jetstream.RePublish{Destination: ">"}},                                                            // want `stream config: republish destination ">" forms a cycle with subject "ORDERS"`
	{Name: "s", Subjects: []string{"in.>"}, SubjectTransform: &jetstream.SubjectTransformConfig{Source: "in.>", Destination: "out.>"}, RePublish: &jetstream.RePublish{Source: ">", Destination: ">"}},
	{Name: "s", Subjects: []string{"in.>", "c.>"}, SubjectTransform: &jetstream.SubjectTransformConfig{Source: "in.>", Destination: "out.>"}, RePublish: &jetstream.RePublish{Destination: ">"}}, // want `stream config: republish destination ">" forms a cycle with subject "in\.>"`
	{Name: "s", Subjects: []string{"orders.>"}, RePublish: &jetstream.RePublish{Source: "orders.>", Destination: "repub.orders.>"}},
	{Name: "s", Sources: []*jetstream.StreamSource{{Name: "A"}}, RePublish: &jetstream.RePublish{Destination: ">"}},
}

// KeyValue parents: checks on the nested literal alone apply, parent checks do not.
var _ = []jetstream.KeyValueConfig{
	{Bucket: "b", RePublish: &jetstream.RePublish{Source: "orders.*", Destination: "repub.$2"}},                                                                           // want `stream config: republish with transform from "orders\.\*" to "repub\.\$2" not valid`
	{Bucket: "b", Sources: []*jetstream.StreamSource{{Name: "other", SubjectTransforms: []jetstream.SubjectTransformConfig{{Source: "events.>.*", Destination: "x.>"}}}}}, // want `stream config: subject transform source: invalid subject "events\.>\.\*"`
	{Bucket: "b", Sources: []*jetstream.StreamSource{{Name: ""}}},
	{Bucket: "b", Mirror: &jetstream.StreamSource{Name: "A", SubjectTransforms: []jetstream.SubjectTransformConfig{{Source: "orders.*.new", Destination: "x.$1"}, {Source: "orders.eu.*", Destination: "y.$1"}}}}, // want `stream config: subject transform sources "orders\.\*\.new" and "orders\.eu\.\*" can not overlap`
}

func republishVar(ctx context.Context, js jetstream.JetStream) {
	rp := &jetstream.RePublish{Source: "orders.*", Destination: "repub.$2"} // want `stream config: republish with transform from "orders\.\*" to "repub\.\$2" not valid`
	js.CreateStream(ctx, jetstream.StreamConfig{Name: "s", Subjects: []string{"orders.*"}, RePublish: rp})
}

func mirrorVar(m *jetstream.StreamSource) jetstream.StreamConfig {
	return jetstream.StreamConfig{Name: "M", Mirror: m}
}

func sourceFilterCleared(ctx context.Context, js jetstream.JetStream) {
	cfg := jetstream.StreamConfig{Name: "s", Sources: []*jetstream.StreamSource{{Name: "A", FilterSubject: "a.>", SubjectTransforms: []jetstream.SubjectTransformConfig{{Source: "a.b", Destination: "c.b"}}}}}
	cfg.Sources[0].FilterSubject = ""
	js.CreateStream(ctx, cfg)
}

func subjectsElementReplaced(ctx context.Context, js jetstream.JetStream) {
	cfg := jetstream.StreamConfig{Name: "s", Subjects: []string{"a", "a"}}
	cfg.Subjects[1] = "b"
	js.CreateStream(ctx, cfg)
}
