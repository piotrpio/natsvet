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
	"go/ast"
	"go/types"
	"time"
)

// Fields is a constant-only view of one composite literal for a config
// rule. Every accessor returns the value and whether it is known: an absent
// field is the type's zero value and known; a field with a non-constant
// value is unknown, which disables any check that reads it. Aliases map a
// canonical field name to the name the matched (legacy) type uses.
type Fields struct {
	Info    *types.Info
	Fields  map[string]ast.Expr
	Aliases map[string]string
}

// NewFields wraps the result of CompositeFields; aliases apply only when
// the matched type is legacy.
func NewFields(info *types.Info, fields map[string]ast.Expr, legacy bool, aliases map[string]string) *Fields {
	f := &Fields{Info: info, Fields: fields}
	if legacy {
		f.Aliases = aliases
	}
	return f
}

// Expr returns the field's value expression and whether it is present.
func (f *Fields) Expr(field string) (ast.Expr, bool) {
	if alias, ok := f.Aliases[field]; ok {
		field = alias
	}
	e, ok := f.Fields[field]
	return e, ok
}

func (f *Fields) Str(field string) (string, bool) {
	e, present := f.Expr(field)
	if !present {
		return "", true
	}
	return ConstString(f.Info, e)
}

func (f *Fields) Int(field string) (int64, bool) {
	e, present := f.Expr(field)
	if !present {
		return 0, true
	}
	return ConstInt(f.Info, e)
}

func (f *Fields) Dur(field string) (time.Duration, bool) {
	e, present := f.Expr(field)
	if !present {
		return 0, true
	}
	return ConstDuration(f.Info, e)
}

func (f *Fields) Bool(field string) (bool, bool) {
	e, present := f.Expr(field)
	if !present {
		return false, true
	}
	return ConstBool(f.Info, e)
}

// Strs returns the constant elements of a []string field and whether every
// element was constant. Absent and nil count as an empty, complete slice; a
// non-literal value is unknown (complete == false, known == false).
func (f *Fields) Strs(field string) (vals []string, complete, known bool) {
	e, present := f.Expr(field)
	if !present {
		return nil, true, true
	}
	if _, ok := f.SliceLen(field); !ok {
		return nil, false, false
	}
	vals, complete = SliceConstStrings(f.Info, e)
	return vals, complete, true
}

// SliceLen returns the element count of a slice field literal; absent or
// nil is 0; a non-literal value is unknown.
func (f *Fields) SliceLen(field string) (int, bool) {
	e, present := f.Expr(field)
	if !present {
		return 0, true
	}
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return 0, e.Name == "nil" && f.Info.Types[e].IsNil()
	case *ast.CompositeLit:
		return len(e.Elts), true
	}
	return 0, false
}

// Durs returns the constant elements of a []time.Duration field literal.
func (f *Fields) Durs(field string) []time.Duration {
	e, present := f.Expr(field)
	if !present {
		return nil
	}
	lit, ok := ast.Unparen(e).(*ast.CompositeLit)
	if !ok {
		return nil
	}
	var vals []time.Duration
	for _, elt := range lit.Elts {
		if d, ok := ConstDuration(f.Info, elt); ok {
			vals = append(vals, d)
		}
	}
	return vals
}

// Enum returns the name of the nats.go enum constant a field holds; zero
// is the name of the type's zero value, used for an absent field or a
// literal 0.
func (f *Fields) Enum(field, zero string) (string, bool) {
	e, present := f.Expr(field)
	if !present {
		return zero, true
	}
	if v, ok := ConstEnum(f.Info, e, JetStream, Core); ok {
		return v, true
	}
	if v, ok := ConstInt(f.Info, e); ok && v == 0 {
		return zero, true
	}
	return "", false
}

// PtrState classifies a pointer-typed field's value.
type PtrState int

const (
	PtrNil     PtrState = iota // absent or literally nil
	PtrSet                     // an address-of expression
	PtrUnknown                 // anything else
)

// Ptr classifies a pointer-typed field.
func (f *Fields) Ptr(field string) PtrState {
	e, present := f.Expr(field)
	if !present {
		return PtrNil
	}
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		if e.Name == "nil" && f.Info.Types[e].IsNil() {
			return PtrNil
		}
	case *ast.UnaryExpr:
		return PtrSet
	}
	return PtrUnknown
}
