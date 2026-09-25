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

package migrate

import (
	"bytes"
	"go/format"
	"testing"
)

// TestImportEditsSorted checks that the imports a step adds go into gofmt's
// groups in sorted order, so a gofmt-clean file stays gofmt-clean.
func TestImportEditsSorted(t *testing.T) {
	const body = "\nfunc f() {\n\t_ = context.Background\n\t_ = jetstream.New\n\t_ = nats.Connect\n\t_ = bufio.NewReader\n\t_ = testing.Short\n\t_ = require.NoError\n\t_ = fmt.Sprint\n}\n"
	tests := []struct {
		name, imports, want string
	}{
		{
			name:    "std group gains context in order",
			imports: "import (\n\t\"bufio\"\n\t\"fmt\"\n\t\"testing\"\n\n\t\"github.com/nats-io/nats.go\"\n\t\"github.com/nats-io/nats.go/jetstream\"\n\t\"github.com/stretchr/testify/require\"\n)\n",
			want:    "import (\n\t\"bufio\"\n\t\"context\"\n\t\"fmt\"\n\t\"testing\"\n\n\t\"github.com/nats-io/nats.go\"\n\t\"github.com/nats-io/nats.go/jetstream\"\n\t\"github.com/stretchr/testify/require\"\n)\n",
		},
		{
			name:    "third-party group gains jetstream in order",
			imports: "import (\n\t\"bufio\"\n\t\"context\"\n\t\"fmt\"\n\t\"testing\"\n\n\t\"github.com/nats-io/nats.go\"\n\t\"github.com/stretchr/testify/require\"\n)\n",
			want:    "import (\n\t\"bufio\"\n\t\"context\"\n\t\"fmt\"\n\t\"testing\"\n\n\t\"github.com/nats-io/nats.go\"\n\t\"github.com/nats-io/nats.go/jetstream\"\n\t\"github.com/stretchr/testify/require\"\n)\n",
		},
		{
			name:    "no std group",
			imports: "import (\n\t\"github.com/nats-io/nats.go\"\n\t\"github.com/nats-io/nats.go/jetstream\"\n\t\"github.com/stretchr/testify/require\"\n)\n\nimport (\n\t\"bufio\"\n\t\"fmt\"\n\t\"testing\"\n)\n",
			want:    "import (\n\t\"context\"\n\n\t\"github.com/nats-io/nats.go\"\n\t\"github.com/nats-io/nats.go/jetstream\"\n\t\"github.com/stretchr/testify/require\"\n)\n\nimport (\n\t\"bufio\"\n\t\"fmt\"\n\t\"testing\"\n)\n",
		},
		{
			name:    "no third-party group",
			imports: "import (\n\t\"bufio\"\n\t\"context\"\n\t\"fmt\"\n\t\"testing\"\n\n\t\"github.com/stretchr/testify/require\"\n)\n\nimport \"github.com/nats-io/nats.go\"\n",
			want:    "import (\n\t\"bufio\"\n\t\"context\"\n\t\"fmt\"\n\t\"testing\"\n\n\t\"github.com/nats-io/nats.go/jetstream\"\n\t\"github.com/stretchr/testify/require\"\n)\n\nimport \"github.com/nats-io/nats.go\"\n",
		},
		{
			name:    "single third-party import becomes a block",
			imports: "import \"github.com/nats-io/nats.go\"\n",
			want:    "import (\n\t\"context\"\n\n\t\"github.com/nats-io/nats.go\"\n\t\"github.com/nats-io/nats.go/jetstream\"\n)\n",
		},
		{
			name:    "single std import becomes a block",
			imports: "import \"fmt\"\n",
			want:    "import (\n\t\"context\"\n\t\"fmt\"\n\n\t\"github.com/nats-io/nats.go\"\n\t\"github.com/nats-io/nats.go/jetstream\"\n)\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := []byte("package p\n\n" + tt.imports + body)
			if f, err := format.Source(src); err != nil || !bytes.Equal(f, src) {
				t.Fatalf("test input is not gofmt-clean:\n%s", src)
			}
			its, err := importEdits(src, nil)
			if err != nil {
				t.Fatal(err)
			}
			for i := range its {
				its[i].file = &srcFile{rel: "p.go"}
			}
			sorted, _ := sortIntents(its)
			got := applyText(src, sorted)
			if want := "package p\n\n" + tt.want + body; string(got) != want {
				t.Errorf("imports:\n%s\nwant:\n%s", got, want)
			}
			if f, err := format.Source(got); err != nil || !bytes.Equal(f, got) {
				t.Errorf("result is not gofmt-clean:\n%s", got)
			}
		})
	}
}
