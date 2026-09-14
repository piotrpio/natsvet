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

package nilheader

import "github.com/nats-io/nats.go"

func pointerLiteral() {
	m := &nats.Msg{Subject: "s"}
	m.Header.Set("X-Id", "1") // want `Header.Set on a nats.Msg literal without Header panics \(nil map\); use nats.NewMsg or set Header: nats.Header\{\}`
}

func valueLiteral() {
	m := nats.Msg{Subject: "s"}
	m.Header.Add("X-Id", "1") // want `Header.Add on a nats.Msg literal without Header panics \(nil map\); use nats.NewMsg or set Header: nats.Header\{\}`
}

func indexAssignment() {
	m := &nats.Msg{Subject: "s"}
	m.Header["X-Id"] = []string{"1"} // want `assignment to m.Header on a nats.Msg literal without Header panics \(nil map\); use nats.NewMsg or set Header: nats.Header\{\}`
}

func reads() {
	m := &nats.Msg{Subject: "s"}
	_ = m.Header["X-Id"]
	_ = m.Header.Get("X-Id")
	_ = m.Header.Values("X-Id")
	m.Header.Del("X-Id")
	delete(m.Header, "X-Id")
	_ = len(m.Header)
}

func newMsg() {
	m := nats.NewMsg("s")
	m.Header.Set("X-Id", "1")
}

func withHeader() {
	m := &nats.Msg{Subject: "s", Header: nats.Header{}}
	m.Header.Set("X-Id", "1")
	n := &nats.Msg{Subject: "s", Header: make(nats.Header)}
	n.Header["X-Id"] = []string{"1"}
}

func assignedLater() {
	m := &nats.Msg{Subject: "s"}
	m.Header = nats.Header{}
	m.Header.Set("X-Id", "1")
}

func parameter(m *nats.Msg) {
	m.Header.Set("X-Id", "1")
}

func reassigned() {
	m := &nats.Msg{Subject: "s"}
	m = nats.NewMsg("s")
	m.Header.Set("X-Id", "1")
}
