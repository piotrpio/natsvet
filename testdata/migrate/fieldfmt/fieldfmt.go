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

// Package fieldfmt holds a handle in an aligned struct field used from
// another package, and a file whose first import sorts before context.
package fieldfmt

import "github.com/nats-io/nats.go"

type Server struct {
	NC   *nats.Conn
	JS   nats.JetStreamContext
	Name string
}

func New(nc *nats.Conn) (*Server, error) {
	js, err := nc.JetStream()
	if err != nil {
		return nil, err
	}
	return &Server{NC: nc, JS: js, Name: "fieldfmt"}, nil
}
