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

package streamconfig

import (
	"fmt"
	"go/ast"
	"go/types"

	"github.com/piotrpio/natsvet/internal/natsapi"
)

// nested runs the checks on SubjectTransformConfig, RePublish and
// StreamSource literals: those that hold for the literal alone at its own
// visit, and those that need the enclosing stream config at the stream
// config's visit.
type nested struct {
	info   *types.Info
	stack  []ast.Node
	report func(at ast.Node, msg string)
}

func (v *nested) fields(lit *ast.CompositeLit, assigned map[string]bool, refs ...natsapi.TypeRef) (*natsapi.Fields, bool) {
	fields, _, ok := natsapi.CompositeFields(v.info, lit, refs...)
	if !ok {
		return nil, false
	}
	return natsapi.NewFields(v.info, fields, false, nil, assigned), true
}

// checkLiteral runs the checks that need only the literal itself.
func (v *nested) checkLiteral(lit *ast.CompositeLit, masker *natsapi.Masker) {
	fields, matched, ok := natsapi.CompositeFields(v.info, lit, nestedRefs...)
	if !ok {
		return
	}
	assigned := masker.Masked(v.stack)
	c := natsapi.NewFields(v.info, fields, false, nil, assigned)
	switch matched.Name {
	case "SubjectTransformConfig":
		if !v.underLegacyKVSources() {
			v.checkTransform(lit, c)
		}
	case "RePublish":
		v.checkRePublish(lit, c)
	case "StreamSource":
		v.checkSource(lit, c, assigned)
	}
}

func (v *nested) checkTransform(lit *ast.CompositeLit, c *natsapi.Fields) {
	src, srcOK := c.Str("Source")
	if srcOK && src != "" && !natsapi.IsValidSubject(src) {
		v.report(lit, fmt.Sprintf("subject transform source: invalid subject %q", src))
		return
	}
	dest, destOK := c.Str("Destination")
	if !srcOK || !destOK {
		return
	}
	if err := natsapi.ValidateMapping(src, dest); err != nil {
		v.report(lit, fmt.Sprintf("subject transform from %q to %q: %v", src, dest, err))
	}
}

func (v *nested) checkRePublish(lit *ast.CompositeLit, c *natsapi.Fields) {
	src, srcOK := c.Str("Source")
	dest, destOK := c.Str("Destination")
	if !srcOK || !destOK || dest == "" {
		return
	}
	if src == "" {
		src = ">"
	}
	if natsapi.SubjectTransformErr(src, dest) != nil {
		v.report(lit, fmt.Sprintf("republish with transform from %q to %q not valid", src, dest))
	}
}

func (v *nested) checkSource(lit *ast.CompositeLit, c *natsapi.Fields, assigned map[string]bool) {
	if d, ok := c.Str("Domain"); ok && d != "" && c.Ptr("External") == natsapi.PtrSet {
		v.report(lit, "domain and external are both set")
	}
	if !v.underLegacyKVSources() {
		filter, filterOK := c.Str("FilterSubject")
		if n, ok := c.SliceLen("SubjectTransforms"); ok && n > 0 && filterOK && filter != "" {
			v.report(lit, "a source or mirror with subject transforms cannot also have a single subject filter")
		}
		v.checkOverlap(lit, c, assigned)
	}
	v.checkConsumer(lit, c, assigned)
}

// checkOverlap mirrors the pairwise transform-source checks: subset match
// for sources, collision for a mirror.
func (v *nested) checkOverlap(lit *ast.CompositeLit, c *natsapi.Fields, assigned map[string]bool) {
	lits, _ := c.Lits("SubjectTransforms")
	var sources []string
	for _, tl := range lits {
		tc, ok := v.fields(tl, assigned, transformRefs...)
		if !ok {
			continue
		}
		if s, ok := tc.Str("Source"); ok && (s == "" || natsapi.IsValidSubject(s)) {
			sources = append(sources, s)
		}
	}
	mirror := v.isMirror()
	for i, a := range sources {
		for _, b := range sources[i+1:] {
			if natsapi.SubjectIsSubsetMatch(a, b) || natsapi.SubjectIsSubsetMatch(b, a) || (mirror && natsapi.SubjectsCollide(a, b)) {
				v.report(lit, fmt.Sprintf("subject transform sources %q and %q can not overlap", a, b))
			}
		}
	}
}

func (v *nested) checkConsumer(lit *ast.CompositeLit, c *natsapi.Fields, assigned map[string]bool) {
	lits, _ := c.Lits("Consumer")
	if len(lits) != 1 {
		return
	}
	cc, ok := v.fields(lits[0], assigned, consumerRefs...)
	if !ok {
		return
	}
	invalid := func(reason string) {
		v.report(lit, "stream source consumer config is invalid: "+reason)
	}
	if n, ok := cc.Str("Name"); ok && invalidAssetName(n) {
		invalid(`consumer name is required and can not contain '.', '*', '>', '\', '/' or whitespace`)
	}
	if s, ok := cc.Str("DeliverSubject"); ok && (!natsapi.SubjectIsLiteral(s) || !natsapi.IsValidSubject(s)) {
		invalid("deliver subject must be a valid literal subject")
	}
	if seq, ok := c.Int("OptStartSeq"); (ok && seq != 0) || c.Ptr("OptStartTime") == natsapi.PtrSet {
		invalid("a start sequence or start time can not be set")
	}
	if f, ok := c.Str("FilterSubject"); ok && f != "" {
		invalid("a filter subject can not be set")
	}
}

// checkParent runs the checks on nested literals that need the enclosing
// stream config.
func (v *nested) checkParent(c *natsapi.Fields) {
	v.checkSourceNames(c)
	v.checkMirrorName(c)
	v.checkCycle(c)
}

// checkSourceNames mirrors `src == nil || !isValidAssetName(src.Name)`.
func (v *nested) checkSourceNames(c *natsapi.Fields) {
	e, ok := c.Expr("Sources")
	if !ok || c.Assigned["Sources"] {
		return
	}
	slice, ok := ast.Unparen(e).(*ast.CompositeLit)
	if !ok {
		return
	}
	for _, elt := range slice.Elts {
		if id, ok := ast.Unparen(elt).(*ast.Ident); ok && id.Name == "nil" && v.info.Types[id].IsNil() {
			v.report(elt, "sourced stream name is invalid")
			continue
		}
		lit := natsapi.CompositeOf(elt)
		if lit == nil {
			continue
		}
		if sc, ok := v.fields(lit, c.Assigned, sourceRefs...); ok {
			if n, ok := sc.Str("Name"); ok && invalidAssetName(n) {
				v.report(lit, "sourced stream name is invalid")
			}
		}
	}
}

// checkMirrorName mirrors isValidAssetName on the mirror name, which the
// server skips when the mirror has an External (nats.go sets one from
// Domain).
func (v *nested) checkMirrorName(c *natsapi.Fields) {
	lits, _ := c.Lits("Mirror")
	if len(lits) != 1 {
		return
	}
	mc, ok := v.fields(lits[0], c.Assigned, sourceRefs...)
	if !ok || mc.Ptr("External") != natsapi.PtrNil {
		return
	}
	if d, ok := mc.Str("Domain"); !ok || d != "" {
		return
	}
	if n, ok := mc.Str("Name"); ok && invalidAssetName(n) {
		v.report(lits[0], "mirrored stream name is invalid")
	}
}

// checkCycle mirrors the republish cycle check, including the default
// subject and the implicit republish the server derives from a subject
// transform.
func (v *nested) checkCycle(c *natsapi.Fields) {
	lits, _ := c.Lits("RePublish")
	if len(lits) != 1 {
		return
	}
	rc, ok := v.fields(lits[0], c.Assigned, republishRefs...)
	if !ok {
		return
	}
	src, srcOK := rc.Str("Source")
	dest, destOK := rc.Str("Destination")
	if !srcOK || !destOK {
		return
	}
	if src == "" {
		src = ">"
	}
	subjects, complete, known := c.Strs("Subjects")
	count, _ := c.SliceLen("Subjects")
	if !known {
		return
	}
	if count == 0 {
		sources, sourcesOK := c.SliceLen("Sources")
		if !sourcesOK || c.Ptr("Mirror") == natsapi.PtrUnknown {
			return
		}
		if sources > 0 || c.Ptr("Mirror") == natsapi.PtrSet {
			return
		}
		name, ok := c.Str("Name")
		if !ok {
			return
		}
		subjects, complete, count = []string{name}, true, 1
	}
	if src == ">" && dest == ">" && count == 1 && c.Ptr("SubjectTransform") != natsapi.PtrNil {
		if !complete {
			return
		}
		tls, _ := c.Lits("SubjectTransform")
		if len(tls) != 1 {
			return
		}
		tc, ok := v.fields(tls[0], c.Assigned, transformRefs...)
		if !ok {
			return
		}
		tsrc, tsrcOK := tc.Str("Source")
		tdest, tdestOK := tc.Str("Destination")
		if !tsrcOK || !tdestOK {
			return
		}
		if subjects[0] == tsrc {
			dest = tdest
		}
	}
	for _, s := range subjects {
		if natsapi.SubjectsCollide(dest, s) {
			v.report(lits[0], fmt.Sprintf("republish destination %q forms a cycle with subject %q", dest, s))
			return
		}
	}
}

// parentField returns the field name under which the literal at the top of
// the stack is written and the composite literal holding that field,
// looking through &, parentheses and an enclosing slice literal.
func parentField(info *types.Info, stack []ast.Node) (string, *ast.CompositeLit) {
	i := len(stack) - 1
	for i > 0 {
		switch p := stack[i-1].(type) {
		case *ast.UnaryExpr, *ast.ParenExpr:
		case *ast.CompositeLit:
			switch info.TypeOf(p).Underlying().(type) {
			case *types.Slice, *types.Array:
			default:
				return "", nil
			}
		case *ast.KeyValueExpr:
			key, ok := p.Key.(*ast.Ident)
			if !ok || i < 2 {
				return "", nil
			}
			parent, ok := stack[i-2].(*ast.CompositeLit)
			if !ok {
				return "", nil
			}
			return key.Name, parent
		default:
			return "", nil
		}
		i--
	}
	return "", nil
}

// isMirror reports whether the literal is written as the Mirror of a
// stream or KeyValue config literal.
func (v *nested) isMirror() bool {
	field, parent := parentField(v.info, v.stack)
	if field != "Mirror" || parent == nil {
		return false
	}
	_, _, ok := natsapi.CompositeFields(v.info, parent, jsConfig, legacyConfig, jsKV, legacyKV)
	return ok
}

// underLegacyKVSources reports whether the literal sits inside the Sources
// of a legacy nats.KeyValueConfig literal, whose transforms nats.go
// replaces before submission.
func (v *nested) underLegacyKVSources() bool {
	for i := len(v.stack) - 1; i > 0; i-- {
		kv, ok := v.stack[i].(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if key, ok := kv.Key.(*ast.Ident); !ok || key.Name != "Sources" {
			continue
		}
		if parent, ok := v.stack[i-1].(*ast.CompositeLit); ok {
			if _, _, ok := natsapi.CompositeFields(v.info, parent, legacyKV); ok {
				return true
			}
		}
	}
	return false
}
