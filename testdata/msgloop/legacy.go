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

import "github.com/nats-io/nats.go"

func legacyFetchBatch(sub *nats.Subscription) {
	msgs, err := sub.FetchBatch(10)
	if err != nil {
		return
	}
	for msg := range msgs.Messages() { // want `FetchBatch result ranged without checking Error\(\); a failed fetch looks like an empty batch`
		msg.Ack()
	}
}

func legacyChecked(sub *nats.Subscription) error {
	msgs, err := sub.FetchBatch(10)
	if err != nil {
		return err
	}
	for msg := range msgs.Messages() {
		msg.Ack()
	}
	return msgs.Error()
}
