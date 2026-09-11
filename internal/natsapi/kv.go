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
	"regexp"
	"strings"
)

// KeyValue validation ported from nats.go jetstream/kv.go (bucketValid,
// keyValid, searchKeyValid). The same bucket rule governs object stores.

var (
	validBucketRe    = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)
	validKeyRe       = regexp.MustCompile(`^[-/_=\.a-zA-Z0-9]+$`)
	validSearchKeyRe = regexp.MustCompile(`^[-/_=\.a-zA-Z0-9*]*[>]?$`)
)

// BucketValid reports whether bucket is a valid KeyValue or ObjectStore
// bucket name.
func BucketValid(bucket string) bool {
	return validBucketRe.MatchString(bucket)
}

// KeyValid reports whether key is accepted by Get, Put, Create, Update,
// Delete and Purge.
func KeyValid(key string) bool {
	return keyShapeValid(key) && validKeyRe.MatchString(key)
}

// SearchKeyValid reports whether key is accepted as a Watch, History or
// ListKeysFiltered filter: like KeyValid but wildcard tokens are allowed.
func SearchKeyValid(key string) bool {
	return keyShapeValid(key) && validSearchKeyRe.MatchString(key)
}

func keyShapeValid(key string) bool {
	return len(key) != 0 && key[0] != '.' && key[len(key)-1] != '.' && !strings.Contains(key, "..")
}
