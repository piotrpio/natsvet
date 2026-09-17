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

import "go/types"

// PackageUses reports whether any identifier in the analyzed package resolves
// to the function pkg.name (recv empty) or the method recv.name declared in
// pkg. It looks at uses only, never declarations, and sees exactly the files
// the driver type-checked together: the package, plus its test files when
// the test variant is analyzed, and never a dependency.
func PackageUses(info *types.Info, pkg Pkg, recv, name string) bool {
	for _, obj := range info.Uses {
		fn, ok := obj.(*types.Func)
		if !ok || fn.Name() != name {
			continue
		}
		if recv == "" && IsFunc(fn, pkg, name) || recv != "" && IsMethod(fn, pkg, recv, name) {
			return true
		}
	}
	return false
}
