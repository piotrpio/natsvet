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

package handle

import (
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/micro"
)

func serviceBlank(nc *nats.Conn, cfg micro.Config) {
	_, err := micro.AddService(nc, cfg) // want `Service from AddService discarded; the service can never be stopped and keeps answering until the connection closes`
	_ = err
}

func serviceKept(nc *nats.Conn, cfg micro.Config) {
	svc, err := micro.AddService(nc, cfg)
	if err != nil {
		return
	}
	defer svc.Stop()
}
