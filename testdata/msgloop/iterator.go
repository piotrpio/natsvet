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

package msgloop

import (
	"context"
	"errors"
	"log"
	"os"
	"testing"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func logAndContinue(it jetstream.MessagesContext) {
	for {
		msg, err := it.Next()
		if err != nil { // want `loop continues on every Next error; after Stop or Drain, Next returns ErrMsgIteratorClosed on every call and the loop never exits`
			log.Println(err)
			continue
		}
		msg.Ack()
	}
}

func elseBranch(it jetstream.MessagesContext) {
	for {
		msg, err := it.Next()
		if err != nil { // want `the loop never exits`
			log.Println(err)
		} else {
			msg.Ack()
		}
	}
}

func ifInit(it jetstream.MessagesContext) {
	for {
		if msg, err := it.Next(); err != nil { // want `the loop never exits`
			log.Println(err)
			continue
		} else {
			msg.Ack()
		}
	}
}

func backoff(it jetstream.MessagesContext) {
	for {
		msg, err := it.Next()
		if err != nil { // want `the loop never exits`
			log.Println(err)
			time.Sleep(time.Second)
			continue
		}
		msg.Ack()
	}
}

func assignForm(it jetstream.MessagesContext) {
	var msg jetstream.Msg
	var err error
	for {
		msg, err = it.Next()
		if err != nil { // want `the loop never exits`
			continue
		}
		msg.Ack()
	}
}

func labeledContinue(its []jetstream.MessagesContext) {
outer:
	for _, it := range its {
		for {
			msg, err := it.Next()
			if err != nil {
				continue outer
			}
			msg.Ack()
		}
	}
}

func breakOnClosed(it jetstream.MessagesContext) {
	for {
		msg, err := it.Next()
		if err != nil {
			if errors.Is(err, jetstream.ErrMsgIteratorClosed) {
				break
			}
			continue
		}
		msg.Ack()
	}
}

func closedHandledEarlier(it jetstream.MessagesContext) {
	for {
		msg, err := it.Next()
		if errors.Is(err, jetstream.ErrMsgIteratorClosed) {
			return
		}
		if err != nil {
			continue
		}
		msg.Ack()
	}
}

func returnOnError(it jetstream.MessagesContext) error {
	for {
		msg, err := it.Next()
		if err != nil {
			return err
		}
		msg.Ack()
	}
}

func fatalOnError(it jetstream.MessagesContext) {
	for {
		msg, err := it.Next()
		if err != nil {
			log.Fatal(err)
		}
		msg.Ack()
	}
}

func testFatal(t *testing.T, it jetstream.MessagesContext) {
	for {
		msg, err := it.Next()
		if err != nil {
			t.Fatalf("next: %v", err)
		}
		msg.Ack()
	}
}

func exitOnError(it jetstream.MessagesContext) {
	for {
		msg, err := it.Next()
		if err != nil {
			os.Exit(1)
		}
		msg.Ack()
	}
}

func conditionalLoop(ctx context.Context, it jetstream.MessagesContext) {
	for ctx.Err() == nil {
		msg, err := it.Next()
		if err != nil {
			continue
		}
		msg.Ack()
	}
}

func rangeLoop(it jetstream.MessagesContext) {
	for range 10 {
		msg, err := it.Next()
		if err != nil {
			continue
		}
		msg.Ack()
	}
}

func oneShotNext(cons jetstream.Consumer) {
	for {
		msg, err := cons.Next()
		if err != nil {
			continue
		}
		msg.Ack()
	}
}

func timeoutOnly(it jetstream.MessagesContext) error {
	for {
		msg, err := it.Next(jetstream.NextMaxWait(time.Second))
		if err != nil {
			if errors.Is(err, nats.ErrTimeout) {
				continue
			}
			return err
		}
		msg.Ack()
	}
}
