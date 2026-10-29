package models

import (
	"reflect"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
)

// PrepareForEncode returns v with every bare time.Time and *time.Time replaced
// by a CustomDateTime, so that the CBOR encoder emits tag 12 ([seconds, nanos])
// instead of tag 0 (an RFC3339 string), which SurrealDB 4.0 rejects.
//
// fxamacker/cbor hard-codes the encoding of time.Time ahead of any Marshaler
// lookup and refuses to register it in a TagSet, so the substitution has to
// happen before encoding. The walk covers structs, maps, slices, arrays,
// pointers and interface values.
//
// Values are returned unchanged (same value, no copy) when they contain no
// time.Time. Types that implement cbor.Marshaler, cbor.Tag, cbor.RawTag and
// all types declared in this package are left alone, so CustomDateTime is
// never wrapped twice and its zero-time behaviour (tag 6) is preserved.
//
// A struct is rebuilt as an anonymous struct type (reflect.StructOf) that keeps
// the field names and tags. Unexported fields are dropped, as the encoder
// ignores them anyway, and untagged embedded structs are flattened into the
// parent, which matches the encoder's own flattening. A nil embedded pointer
// therefore encodes its fields as zero values. Self-referential struct types
// cannot be rebuilt, so a time.Time reached through the recursive edge is left
// as is.
func PrepareForEncode(v any) any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	if !hasTime(rv) {
		return v
	}
	p := planFor(rv.Type(), nil)
	if p == nil {
		return v
	}
	return p.conv(rv).Interface()
}

type plan struct {
	out  reflect.Type
	conv func(reflect.Value) reflect.Value
}

var (
	timeType      = reflect.TypeOf(time.Time{})
	timePtrType   = reflect.TypeOf((*time.Time)(nil))
	customDTType  = reflect.TypeOf(CustomDateTime{})
	marshalerType = reflect.TypeOf((*cbor.Marshaler)(nil)).Elem()
	cborTagType   = reflect.TypeOf(cbor.Tag{})
	cborRawTag    = reflect.TypeOf(cbor.RawTag{})
	modelsPkgPath = reflect.TypeOf(CustomDateTime{}).PkgPath()

	planCache sync.Map // reflect.Type -> *plan (nil plan stored as noPlan)
	mayCache  sync.Map // reflect.Type -> bool
	noPlan    = &plan{}
)

// leaveAlone reports whether the encoder must see values of t untouched.
func leaveAlone(t reflect.Type) bool {
	if t == cborTagType || t == cborRawTag {
		return true
	}
	if t.PkgPath() == modelsPkgPath && t != customDTType {
		return true
	}
	if t == customDTType {
		return true
	}
	return t.Implements(marshalerType) || reflect.PointerTo(t).Implements(marshalerType)
}

// mayContainTime is a static check: can a value of type t hold a time.Time?
// Interfaces answer true, as their dynamic content is unknown.
func mayContainTime(t reflect.Type) bool {
	if c, ok := mayCache.Load(t); ok {
		return c.(bool)
	}
	r := mayContain(t, map[reflect.Type]bool{})
	mayCache.Store(t, r)
	return r
}

func mayContain(t reflect.Type, seen map[reflect.Type]bool) bool {
	if t == timeType {
		return true
	}
	if leaveAlone(t) || seen[t] {
		return false
	}
	switch t.Kind() {
	case reflect.Interface:
		return true
	case reflect.Pointer, reflect.Slice, reflect.Array:
		seen[t] = true
		return mayContain(t.Elem(), seen)
	case reflect.Map:
		seen[t] = true
		return mayContain(t.Key(), seen) || mayContain(t.Elem(), seen)
	case reflect.Struct:
		seen[t] = true
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" && !f.Anonymous {
				continue
			}
			if mayContain(f.Type, seen) {
				return true
			}
		}
	}
	return false
}

// hasTime reports whether v actually holds a time.Time, looking through
// interfaces. It never allocates, and skips whole subtrees whose static type
// cannot hold a time.
func hasTime(v reflect.Value) bool {
	t := v.Type()
	if t == timeType {
		return true
	}
	if !mayContainTime(t) {
		return false
	}
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		return !v.IsNil() && hasTime(v.Elem())
	case reflect.Slice, reflect.Array:
		if !mayContainTime(t.Elem()) {
			return false
		}
		for i := 0; i < v.Len(); i++ {
			if hasTime(v.Index(i)) {
				return true
			}
		}
	case reflect.Map:
		it := v.MapRange()
		for it.Next() {
			if hasTime(it.Key()) || hasTime(it.Value()) {
				return true
			}
		}
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" && !f.Anonymous {
				continue
			}
			if hasTime(v.Field(i)) {
				return true
			}
		}
	}
	return false
}

// planFor returns the conversion for type t, or nil if values of t need none.
func planFor(t reflect.Type, building map[reflect.Type]bool) (p *plan) {
	if c, ok := planCache.Load(t); ok {
		if c.(*plan) == noPlan {
			return nil
		}
		return c.(*plan)
	}
	if building[t] {
		return nil // recursive edge, leave as is
	}
	if building == nil {
		building = map[reflect.Type]bool{}
	}
	building[t] = true
	defer delete(building, t)

	// A plan computed while a recursive edge was cut short is still valid, just
	// less complete, and is deterministic per entry type, so it can be cached.
	defer func() {
		if r := recover(); r != nil {
			p = nil // e.g. reflect.StructOf rejecting a layout: encode unchanged
		}
		if p == nil {
			planCache.Store(t, noPlan)
		} else {
			planCache.Store(t, p)
		}
	}()

	return buildPlan(t, building)
}

func buildPlan(t reflect.Type, building map[reflect.Type]bool) *plan {
	switch {
	case t == timeType:
		return &plan{out: customDTType, conv: func(v reflect.Value) reflect.Value {
			return reflect.ValueOf(CustomDateTime{v.Interface().(time.Time)})
		}}
	case t.Kind() == reflect.Interface:
		return &plan{out: t, conv: func(v reflect.Value) reflect.Value {
			if v.IsNil() {
				return v
			}
			e := v.Elem()
			if !hasTime(e) {
				return v
			}
			ep := planFor(e.Type(), nil)
			if ep == nil {
				return v
			}
			c := ep.conv(e)
			if !c.Type().AssignableTo(t) {
				return v
			}
			r := reflect.New(t).Elem()
			r.Set(c)
			return r
		}}
	}
	if leaveAlone(t) || !mayContainTime(t) {
		return nil
	}

	switch t.Kind() {
	case reflect.Pointer:
		ep := planFor(t.Elem(), building)
		if ep == nil {
			return nil
		}
		out := reflect.PointerTo(ep.out)
		return &plan{out: out, conv: func(v reflect.Value) reflect.Value {
			if v.IsNil() {
				return reflect.Zero(out)
			}
			r := reflect.New(ep.out)
			r.Elem().Set(ep.conv(v.Elem()))
			return r
		}}
	case reflect.Slice:
		ep := planFor(t.Elem(), building)
		if ep == nil {
			return nil
		}
		out := reflect.SliceOf(ep.out)
		return &plan{out: out, conv: func(v reflect.Value) reflect.Value {
			if v.IsNil() {
				return reflect.Zero(out)
			}
			r := reflect.MakeSlice(out, v.Len(), v.Len())
			for i := 0; i < v.Len(); i++ {
				r.Index(i).Set(ep.conv(v.Index(i)))
			}
			return r
		}}
	case reflect.Array:
		ep := planFor(t.Elem(), building)
		if ep == nil {
			return nil
		}
		out := reflect.ArrayOf(t.Len(), ep.out)
		return &plan{out: out, conv: func(v reflect.Value) reflect.Value {
			r := reflect.New(out).Elem()
			for i := 0; i < v.Len(); i++ {
				r.Index(i).Set(ep.conv(v.Index(i)))
			}
			return r
		}}
	case reflect.Map:
		kp, vp := planFor(t.Key(), building), planFor(t.Elem(), building)
		if kp == nil && vp == nil {
			return nil
		}
		kt, vt := t.Key(), t.Elem()
		if kp != nil {
			kt = kp.out
		}
		if vp != nil {
			vt = vp.out
		}
		out := reflect.MapOf(kt, vt)
		return &plan{out: out, conv: func(v reflect.Value) reflect.Value {
			if v.IsNil() {
				return reflect.Zero(out)
			}
			r := reflect.MakeMapWithSize(out, v.Len())
			it := v.MapRange()
			for it.Next() {
				k, e := it.Key(), it.Value()
				if kp != nil {
					k = kp.conv(k)
				}
				if vp != nil {
					e = vp.conv(e)
				}
				r.SetMapIndex(k, e)
			}
			return r
		}}
	case reflect.Struct:
		return buildStructPlan(t, building)
	}
	return nil
}

type fieldPlan struct {
	index []int
	p     *plan
}

func buildStructPlan(t reflect.Type, building map[reflect.Type]bool) *plan {
	var fields []reflect.StructField
	var srcs []fieldPlan
	if !collectFields(t, nil, building, &fields, &srcs) {
		return nil
	}
	out := reflect.StructOf(fields)
	return &plan{out: out, conv: func(v reflect.Value) reflect.Value {
		r := reflect.New(out).Elem()
		for i, s := range srcs {
			fv, ok := fieldByIndex(v, s.index)
			if !ok {
				continue // nil embedded pointer: leave the zero value
			}
			if s.p != nil {
				fv = s.p.conv(fv)
			}
			r.Field(i).Set(fv)
		}
		return r
	}}
}

// collectFields lists the encodable fields of struct type t, flattening
// untagged embedded structs. It reports whether any field needs conversion.
func collectFields(t reflect.Type, prefix []int, building map[reflect.Type]bool,
	fields *[]reflect.StructField, srcs *[]fieldPlan) bool {
	changed := false
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		idx := append(append([]int(nil), prefix...), i)

		if f.Anonymous {
			et := f.Type
			if et.Kind() == reflect.Pointer {
				et = et.Elem()
			}
			_, tagged := f.Tag.Lookup("cbor")
			if !tagged {
				_, tagged = f.Tag.Lookup("json")
			}
			if et.Kind() == reflect.Struct && !tagged && !leaveAlone(f.Type) {
				if collectFields(et, idx, building, fields, srcs) {
					changed = true
				}
				continue
			}
		}
		if f.PkgPath != "" {
			continue // unexported: never encoded
		}
		fp := planFor(f.Type, building)
		ft := f.Type
		if fp != nil {
			ft = fp.out
			changed = true
		}
		*fields = append(*fields, reflect.StructField{Name: f.Name, Type: ft, Tag: f.Tag})
		*srcs = append(*srcs, fieldPlan{index: idx, p: fp})
	}
	return changed
}

func fieldByIndex(v reflect.Value, index []int) (reflect.Value, bool) {
	for i, x := range index {
		if i > 0 && v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return reflect.Value{}, false
			}
			v = v.Elem()
		}
		v = v.Field(x)
	}
	return v, true
}
