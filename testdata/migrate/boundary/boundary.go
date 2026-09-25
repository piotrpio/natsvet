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

package boundary

import (
	"github.com/nats-io/nats.go"

	"natsvet/testdata/migrateext/lib"
)

func Hand(nc *nats.Conn) error {
	js, err := nc.JetStream()
	if err != nil {
		return err
	}
	lib.Use(js)
	return js.DeleteStream("TMP")
}
