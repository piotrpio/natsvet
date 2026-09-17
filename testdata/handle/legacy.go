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

import "github.com/nats-io/nats.go"

func legacyWatch(kv nats.KeyValue, os nats.ObjectStore) {
	_, err := kv.WatchAll()                  // want `KeyWatcher from WatchAll discarded; the watcher can never be stopped and its subscription lives as long as the connection`
	_, err = kv.Watch("orders.*")            // want `KeyWatcher from Watch discarded`
	_, err = kv.WatchFiltered([]string{"a"}) // want `KeyWatcher from WatchFiltered discarded`
	_, err = os.Watch()                      // want `ObjectWatcher from Watch discarded`
	_ = err
}

func legacyWatcherKept(kv nats.KeyValue) {
	w, err := kv.WatchAll()
	if err != nil {
		return
	}
	defer w.Stop()
}

func subscription(nc *nats.Conn) {
	_, err := nc.Subscribe("orders", func(*nats.Msg) {})
	_ = err
}
