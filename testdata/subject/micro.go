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

package subject

import "github.com/nats-io/nats.go/micro"

var _ = []micro.EndpointConfig{
	{Subject: "svc. echo"},                   // want `subject "svc. echo" is invalid: contains whitespace`
	{Subject: "svc.echo", QueueGroup: "q 1"}, // want `queue group "q 1" contains whitespace`
	{Subject: "svc.*"},
	{Subject: "svc.echo", QueueGroup: "workers"},
}

func endpoints(svc micro.Service) {
	_ = svc.AddEndpoint("echo", nil, micro.WithEndpointSubject("svc..echo")) // want `subject "svc..echo" is invalid: empty token`
	_ = svc.AddEndpoint("echo", nil, micro.WithEndpointSubject("svc.echo"))
}
