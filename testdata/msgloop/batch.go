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
	"log"

	"github.com/nats-io/nats.go/jetstream"
)

func rangeWithoutError(cons jetstream.Consumer) {
	msgs, err := cons.Fetch(10)
	if err != nil {
		return
	}
	for msg := range msgs.Messages() { // want `Fetch result ranged without checking Error\(\); a failed fetch looks like an empty batch`
		msg.Ack()
	}
}

func fetchBytesWithoutError(cons jetstream.Consumer) {
	msgs, err := cons.FetchBytes(1024)
	if err != nil {
		return
	}
	for msg := range msgs.Messages() { // want `FetchBytes result ranged without checking Error\(\)`
		msg.Ack()
	}
}

func fetchNoWaitWithoutError(cons jetstream.Consumer) {
	msgs, err := cons.FetchNoWait(10)
	if err != nil {
		return
	}
	for range msgs.Messages() { // want `FetchNoWait result ranged without checking Error\(\)`
	}
}

func errorAfterRange(cons jetstream.Consumer) error {
	msgs, err := cons.Fetch(10)
	if err != nil {
		return err
	}
	for msg := range msgs.Messages() {
		msg.Ack()
	}
	if err := msgs.Error(); err != nil {
		return err
	}
	return nil
}

func errorInDefer(cons jetstream.Consumer) {
	msgs, err := cons.Fetch(10)
	if err != nil {
		return
	}
	defer func() {
		if err := msgs.Error(); err != nil {
			log.Println(err)
		}
	}()
	for msg := range msgs.Messages() {
		msg.Ack()
	}
}

func process(jetstream.MessageBatch) {}

func passedElsewhere(cons jetstream.Consumer) {
	msgs, err := cons.Fetch(10)
	if err != nil {
		return
	}
	process(msgs)
	for msg := range msgs.Messages() {
		msg.Ack()
	}
}

func parameter(batch jetstream.MessageBatch) {
	for msg := range batch.Messages() {
		msg.Ack()
	}
}

func selectReceive(cons jetstream.Consumer, done chan struct{}) {
	msgs, err := cons.Fetch(1)
	if err != nil {
		return
	}
	select {
	case msg := <-msgs.Messages():
		msg.Ack()
	case <-done:
	}
}

func reassigned(cons jetstream.Consumer) {
	msgs, err := cons.Fetch(10)
	if err != nil {
		return
	}
	msgs, err = cons.Fetch(10)
	if err != nil {
		return
	}
	for msg := range msgs.Messages() {
		msg.Ack()
	}
}
