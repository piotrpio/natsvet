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

package complete

import (
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

var data []byte

type publisher struct {
	js jetstream.JetStream
}

func (p *publisher) publish() {
	_, err := p.js.PublishAsync("orders.new", data)
	_ = err
}

func (p *publisher) Close() {
	select {
	case <-p.js.PublishAsyncComplete():
	case <-time.After(5 * time.Second):
	}
}
