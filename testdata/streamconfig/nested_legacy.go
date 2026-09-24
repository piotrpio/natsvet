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

import "github.com/nats-io/nats.go"

var _ = []nats.SubjectTransformConfig{
	{Source: "events..a", Destination: "x"},                    // want `stream config: subject transform source: invalid subject "events\.\.a"`
	{Source: "events.*", Destination: "events.{{split(3,1)}}"}, // want `wildcard index out of range in {{split\(3,1\)}}: \[3\]$`
}

var _ = []nats.RePublish{
	{Destination: "repub.>"},
	{Source: "orders.*", Destination: "repub.$2"}, // want `stream config: republish with transform from "orders\.\*" to "repub\.\$2" not valid`
}

var _ = []*nats.StreamSource{
	{Name: "A", FilterSubject: "a.>", SubjectTransforms: []nats.SubjectTransformConfig{{Source: "a.b", Destination: "c.b"}}}, // want `stream config: a source or mirror with subject transforms cannot also have a single subject filter`
	{Name: "A", Domain: "hub", External: &nats.ExternalStream{APIPrefix: "$JS.hub.API"}},                                     // want `stream config: domain and external are both set`
}

var _ = []nats.StreamConfig{
	{Name: "s", Sources: []*nats.StreamSource{{Name: "a b"}}},                                                  // want `stream config: sourced stream name is invalid`
	{Name: "s", Subjects: []string{"a.>"}, RePublish: &nats.RePublish{Source: "a.>", Destination: "a.copy.>"}}, // want `stream config: republish destination "a\.copy\.>" forms a cycle with subject "a\.>"`
}

// Legacy CreateKeyValue replaces a source's transforms before submission.
var _ = nats.KeyValueConfig{
	Bucket:  "b",
	Sources: []*nats.StreamSource{{Name: "other", FilterSubject: "o.>", SubjectTransforms: []nats.SubjectTransformConfig{{Source: "events.>.*", Destination: "x.>"}, {Source: "events.>", Destination: "y.>"}}}},
}
