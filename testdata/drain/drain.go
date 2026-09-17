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

package drain

import (
	"log"

	"github.com/nats-io/nats.go"
)

var done chan struct{}

func adjacent(nc *nats.Conn) {
	nc.Drain()
	nc.Close() // want `Close immediately after Drain aborts the drain; wait for the ClosedHandler instead`
}

func ifInit(nc *nats.Conn) error {
	if err := nc.Drain(); err != nil {
		return err
	}
	nc.Close() // want `Close immediately after Drain aborts the drain; wait for the ClosedHandler instead`
	return nil
}

func assignedCheck(nc *nats.Conn) {
	err := nc.Drain()
	if err != nil {
		log.Println(err)
	}
	nc.Close() // want `Close immediately after Drain aborts the drain; wait for the ClosedHandler instead`
}

func waitBetween(nc *nats.Conn) {
	nc.Drain()
	<-done
	nc.Close()
}

func differentConns(a, b *nats.Conn) {
	a.Drain()
	b.Close()
}

func checkTouchesConn(nc *nats.Conn) {
	nc.Drain()
	if nc.IsDraining() {
		<-done
	}
	nc.Close()
}

func deferredReturn(nc *nats.Conn) error {
	defer nc.Close()
	return nc.Drain() // want `deferred Close runs as soon as Drain returns and aborts the drain; wait for the ClosedHandler before returning`
}

func deferredTrailing(nc *nats.Conn) {
	defer nc.Close()
	nc.Drain() // want `deferred Close runs as soon as Drain returns and aborts the drain; wait for the ClosedHandler before returning`
}

func deferredThenReturn(nc *nats.Conn) error {
	defer nc.Close()
	nc.Drain() // want `deferred Close runs as soon as Drain returns and aborts the drain; wait for the ClosedHandler before returning`
	return nil
}

func deferredWait(nc *nats.Conn) {
	defer nc.Close()
	nc.Drain()
	<-done
}

func deferredOther(nc, other *nats.Conn) {
	defer other.Close()
	nc.Drain()
}

func noDefer(nc *nats.Conn) {
	nc.Drain()
}

// main in a package other than main is an ordinary function.
func main() {
	nc, _ := nats.Connect("")
	defer nc.Drain()
}
