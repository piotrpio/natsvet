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
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/piotrpio/natsvet/internal/tablegen"
)

func TestTablesUpToDate(t *testing.T) {
	const testdata = "../../testdata"
	if err := tablegen.Available(testdata); err != nil {
		t.Skipf("pinned nats.go not available: %v (run `make testdata-deps`)", err)
	}
	tables, err := tablegen.Collect(testdata)
	if err != nil {
		t.Fatal(err)
	}
	gen, err := tables.Generate()
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][]byte{
		tablegen.HeadersFile: gen.Headers,
		tablegen.LegacyFile:  gen.Legacy,
	} {
		got, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("%s is stale; run `go generate ./internal/natsapi`\n%s", name, firstDiff(string(got), string(want)))
		}
	}
}

// firstDiff describes the first differing line between two texts.
func firstDiff(got, want string) string {
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := range max(len(g), len(w)) {
		var gl, wl string
		if i < len(g) {
			gl = g[i]
		}
		if i < len(w) {
			wl = w[i]
		}
		if gl != wl {
			return fmt.Sprintf("line %d:\n  have: %s\n  want: %s", i+1, gl, wl)
		}
	}
	return ""
}

func TestHeader(t *testing.T) {
	tests := []struct {
		key       string
		canonical string
		first     HeaderConst
		ok        bool
	}{
		{"nats-msg-id", "Nats-Msg-Id", HeaderConst{JetStream, "MsgIDHeader"}, true},
		{"NATS-MSG-ID", "Nats-Msg-Id", HeaderConst{JetStream, "MsgIDHeader"}, true},
		{"Nats-Msg-Id", "Nats-Msg-Id", HeaderConst{JetStream, "MsgIDHeader"}, true},
		{"nats-service-error", "Nats-Service-Error", HeaderConst{Micro, "ErrorHeader"}, true},
		{"x-request-id", "", HeaderConst{}, false},
		{"", "", HeaderConst{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			canonical, consts, ok := Header(tt.key)
			if ok != tt.ok || canonical != tt.canonical {
				t.Fatalf("Header(%q) = (%q, %v, %v), want (%q, _, %v)", tt.key, canonical, consts, ok, tt.canonical, tt.ok)
			}
			if ok && consts[0] != tt.first {
				t.Errorf("first constant = %v, want %v", consts[0], tt.first)
			}
		})
	}
}

const legacySrc = `package nats

type JetStream interface {
	Publish(subj string) error
}

type JetStreamContext interface {
	JetStream
}

type Conn struct{}

func (nc *Conn) JetStream() (JetStreamContext, error) { return nil, nil }
func (nc *Conn) Publish(subj string) error { return nil }

type Msg struct{}

func (m *Msg) Ack() error { return nil }

func Durable(name string) int { return 0 }
func Connect(url string) (*Conn, error) { return nil, nil }

const MsgIdHdr = "Nats-Msg-Id"

func use(nc *Conn, js JetStreamContext, m *Msg) {
	nc.JetStream()
	js.Publish("x")
	nc.Publish("x")
	m.Ack()
	Durable("d")
	Connect("")
	_ = MsgIdHdr
}
`

func TestLegacySymbol(t *testing.T) {
	pkg, info, f := typecheck(t, string(Core), legacySrc)
	tests := []struct {
		name string
		want string
		ok   bool
	}{
		{"nc.JetStream", "nats.Conn.JetStream", true},
		{"js.Publish", "nats.JetStream.Publish", true},
		{"nc.Publish", "", false},
		{"m.Ack", "nats.Msg.Ack", true},
		{"Durable", "nats.Durable", true},
		{"Connect", "", false},
	}
	var inUse []*ast.CallExpr
	for _, c := range calls(f) {
		if _, ok := info.Types[c.Fun]; ok {
			inUse = append(inUse, c)
		}
	}
	if len(inUse) != len(tests) {
		t.Fatalf("found %d calls, want %d", len(inUse), len(tests))
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := LegacySymbol(Callee(info, inUse[i]))
			if got != tt.want || ok != tt.ok {
				t.Errorf("LegacySymbol = (%q, %v), want (%q, %v)", got, ok, tt.want, tt.ok)
			}
		})
	}
	for name, want := range map[string]bool{"JetStreamContext": true, "JetStream": true, "Conn": false, "Msg": false} {
		if _, ok := LegacySymbol(pkg.Scope().Lookup(name)); ok != want {
			t.Errorf("LegacySymbol(%s) ok = %v, want %v", name, ok, want)
		}
	}
	if _, ok := LegacySymbol(pkg.Scope().Lookup("MsgIdHdr")); ok {
		t.Error("constant reported as legacy")
	}
	if _, ok := LegacySymbol(nil); ok {
		t.Error("nil reported as legacy")
	}
}
