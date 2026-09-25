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
	"go/types"
	"slices"
	"strings"
	"sync"
)

// HeaderConst names a nats.go constant that defines a NATS header.
type HeaderConst struct {
	Pkg  Pkg
	Name string
}

var (
	headersOnce  sync.Once
	headersLower map[string]string
	headersKnown []string
)

func indexHeaders() {
	headersLower = make(map[string]string, len(headers)+len(serverHeaders))
	for _, h := range serverHeaders {
		headersLower[strings.ToLower(h)] = h
	}
	for h := range headers {
		headersLower[strings.ToLower(h)] = h
	}
	for _, h := range headersLower {
		headersKnown = append(headersKnown, h)
	}
	slices.Sort(headersKnown)
}

// Header looks key up case-insensitively among the headers nats.go defines
// and the headers nats-server uses, and returns the canonical spelling and
// the nats.go constants that define it, in fix preference order (jetstream,
// nats, micro); a header only nats-server uses has no constants.
func Header(key string) (canonical string, consts []HeaderConst, ok bool) {
	headersOnce.Do(indexHeaders)
	canonical, ok = headersLower[strings.ToLower(key)]
	if !ok {
		return "", nil, false
	}
	return canonical, headers[canonical], true
}

// KnownHeaders returns the canonical names of every header Header knows,
// sorted.
func KnownHeaders() []string {
	headersOnce.Do(indexHeaders)
	return slices.Clone(headersKnown)
}

// LegacySymbols returns every key of the legacy JetStream table ("Type",
// "Func", "Type.Method"), sorted.
func LegacySymbols() []string {
	keys := make([]string, 0, len(legacySymbols))
	for k := range legacySymbols {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// LegacySymbol returns the qualified name of obj ("nats.JetStreamContext",
// "nats.JetStream.Publish", "nats.Durable") when it belongs to the legacy
// JetStream API of package nats.
func LegacySymbol(obj types.Object) (string, bool) {
	if !IsPkg(obj, Core) {
		return "", false
	}
	var key string
	switch o := obj.(type) {
	case *types.TypeName:
		key = o.Name()
	case *types.Func:
		key = o.Name()
		if r := o.Signature().Recv(); r != nil {
			key = receiverName(r.Type()) + "." + key
		}
	default:
		return "", false
	}
	if !legacySymbols[key] {
		return "", false
	}
	return "nats." + key, true
}

// IsDurationAlias reports whether obj is an exported nats.go type declared
// as time.Duration (nats.MaxWait, jetstream.PullExpiry, ...). go/types
// records only the int64 underlying type for such declarations, so the
// answer comes from the generated table.
func IsDurationAlias(obj types.Object) bool {
	tn, ok := obj.(*types.TypeName)
	if !ok || tn.Pkg() == nil {
		return false
	}
	for _, p := range []struct {
		pkg  Pkg
		name string
	}{{JetStream, "jetstream"}, {Core, "nats"}, {Micro, "micro"}} {
		if IsPkg(tn, p.pkg) {
			return durationTypes[p.name+"."+tn.Name()]
		}
	}
	return false
}
