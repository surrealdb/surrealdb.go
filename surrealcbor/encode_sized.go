package surrealcbor

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/surrealdb/surrealdb.go/pkg/models"
)

// Encoding cbor.2.
//
// fxamacker/cbor, which does the encoding, cannot register Go's built-in kinds
// (int8, float32 and so on) in a TagSet, so it cannot emit the sized-number
// tags itself. Instead the cbor.2 encoder lets fxamacker encode the value as
// plain cbor and then walks those bytes alongside the Go value, wrapping each
// item that came from a sized kind in its tag (180 to 188).
//
// The walk follows fxamacker's own output, so struct field names, omitempty,
// embedded structs and so on are whatever fxamacker made of them. It only adds
// a tag where the item at that position decodes to exactly the Go value of a
// sized kind; anywhere it cannot match the bytes to the value (a type that
// encodes itself, a toarray struct, an unexpected shape) it copies the bytes
// unchanged, which leaves those numbers widened, as plain cbor sends them.
// So the walk can fail to tag a number, but cannot tag the wrong one.

var (
	marshalerType  = reflect.TypeOf((*cbor.Marshaler)(nil)).Elem()
	recordIDType   = reflect.TypeOf(models.RecordID{})
	timeType       = reflect.TypeOf(time.Time{})
	bigIntType     = reflect.TypeOf(big.Int{})
	cborTagType    = reflect.TypeOf(cbor.Tag{})
	cborRawTagType = reflect.TypeOf(cbor.RawTag{})
)

const maxSizedWalkDepth = 512

// tagSizedNumbers returns data, the plain cbor encoding of v, with every sized
// number in v tagged for cbor.2. It returns data itself when nothing needed a tag.
func tagSizedNumbers(data []byte, v any) ([]byte, error) {
	w := &sizedWalker{in: data, out: make([]byte, 0, len(data)+8)}
	if err := w.walk(reflect.ValueOf(v), 0); err != nil {
		return nil, fmt.Errorf("cbor.2: tagging sized numbers: %w", err)
	}
	if w.pos != len(data) {
		return nil, errors.New("cbor.2: tagging sized numbers: trailing bytes")
	}
	if !w.tagged {
		return data, nil
	}
	return w.out, nil
}

type sizedWalker struct {
	in     []byte
	pos    int
	out    []byte
	tagged bool
}

// head is a decoded CBOR initial byte and argument.
type head struct {
	major      byte
	info       byte
	arg        uint64
	size       int // bytes taken by the initial byte and argument
	indefinite bool
}

func (w *sizedWalker) peekHead() (head, error) {
	return readHead(w.in, w.pos)
}

func readHead(data []byte, pos int) (head, error) {
	if pos >= len(data) {
		return head{}, errors.New("unexpected end of input")
	}
	b := data[pos]
	h := head{major: b >> 5, info: b & 0x1f, size: 1}
	switch {
	case h.info < 24:
		h.arg = uint64(h.info)
	case h.info <= 27:
		n := 1 << (h.info - 24)
		if pos+1+n > len(data) {
			return head{}, errors.New("unexpected end of input")
		}
		for _, c := range data[pos+1 : pos+1+n] {
			h.arg = h.arg<<8 | uint64(c)
		}
		h.size += n
	case h.info == 31:
		h.indefinite = true
	default:
		return head{}, fmt.Errorf("invalid additional information %d", h.info)
	}
	return h, nil
}

// itemEnd returns the position just past the CBOR item at pos.
//
//nolint:gocyclo // one case per CBOR major type
func itemEnd(data []byte, pos, depth int) (int, error) {
	if depth > maxSizedWalkDepth {
		return 0, errors.New("nesting too deep")
	}
	h, err := readHead(data, pos)
	if err != nil {
		return 0, err
	}
	pos += h.size
	switch h.major {
	case 0, 1:
		return pos, nil
	case 2, 3:
		if h.indefinite {
			for {
				if pos >= len(data) {
					return 0, errors.New("unexpected end of input")
				}
				if data[pos] == 0xff {
					return pos + 1, nil
				}
				if pos, err = itemEnd(data, pos, depth+1); err != nil {
					return 0, err
				}
			}
		}
		if h.arg > uint64(len(data)-pos) { //nolint:gosec // pos <= len(data)
			return 0, errors.New("unexpected end of input")
		}
		return pos + int(h.arg), nil //nolint:gosec // bounded by len(data) just above
	case 4, 5:
		n := h.arg
		if h.major == 5 {
			n *= 2
		}
		if h.indefinite {
			for {
				if pos >= len(data) {
					return 0, errors.New("unexpected end of input")
				}
				if data[pos] == 0xff {
					return pos + 1, nil
				}
				if pos, err = itemEnd(data, pos, depth+1); err != nil {
					return 0, err
				}
			}
		}
		for i := uint64(0); i < n; i++ {
			if pos, err = itemEnd(data, pos, depth+1); err != nil {
				return 0, err
			}
		}
		return pos, nil
	case 6:
		return itemEnd(data, pos, depth+1)
	default: // 7: simple values and floats carry no content after the argument
		return pos, nil
	}
}

// copyItem copies the item at the current position unchanged.
func (w *sizedWalker) copyItem() error {
	end, err := itemEnd(w.in, w.pos, 0)
	if err != nil {
		return err
	}
	w.out = append(w.out, w.in[w.pos:end]...)
	w.pos = end
	return nil
}

// copyHead copies just the initial byte and argument of the item at the current position.
func (w *sizedWalker) copyHead(h head) {
	w.out = append(w.out, w.in[w.pos:w.pos+h.size]...)
	w.pos += h.size
}

// tagItem writes tag (180 to 188) and then copies the item at the current position.
func (w *sizedWalker) tagItem(tag uint64) error {
	w.out = append(w.out, 0xd8, byte(tag)) //nolint:gosec // tags 180 to 188 fit in a byte
	w.tagged = true
	return w.copyItem()
}

// encodesItself reports whether fxamacker hands values of type t to custom
// code (a Marshaler) or encodes them in a way of its own, so the walk must
// treat their bytes as opaque.
func encodesItself(t reflect.Type) bool {
	if t.Implements(marshalerType) || reflect.PointerTo(t).Implements(marshalerType) {
		return true
	}
	switch t {
	case timeType, bigIntType, cborTagType, cborRawTagType:
		return true
	}
	return false
}

//nolint:gocyclo // one case per Go kind
func (w *sizedWalker) walk(v reflect.Value, depth int) error {
	if depth > maxSizedWalkDepth {
		return w.copyItem()
	}

	for v.IsValid() && (v.Kind() == reflect.Interface || v.Kind() == reflect.Pointer) {
		if v.IsNil() {
			return w.copyItem()
		}
		v = v.Elem()
	}
	if !v.IsValid() {
		return w.copyItem()
	}
	switch t := v.Type(); {
	case t == float16Type:
		// Read the bits through reflection: v may be an unexported field.
		return w.walkFloat16(models.Float16FromBits(uint16(v.Field(0).Uint()))) //nolint:gosec // the field is a uint16
	case t == recordIDType:
		return w.walkRecordID(v, depth)
	case encodesItself(t):
		return w.copyItem()
	}

	// A type registered in the encoder's TagSet (models.Set, models.Table and
	// so on) is its tag around its natural encoding: keep the tag and walk on.
	switch v.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map, reflect.Struct:
		for {
			h, err := w.peekHead()
			if err != nil {
				return err
			}
			if h.major != 6 {
				break
			}
			w.copyHead(h)
		}
	}

	return w.walkKind(v, depth)
}

// walkKind walks v, which is neither a pointer nor an interface, by its kind.
func (w *sizedWalker) walkKind(v reflect.Value, depth int) error {
	switch v.Kind() {
	case reflect.Int8:
		return w.walkInt(models.TagI8, v.Int())
	case reflect.Int16:
		return w.walkInt(models.TagI16, v.Int())
	case reflect.Int32:
		return w.walkInt(models.TagI32, v.Int())
	case reflect.Uint8:
		return w.walkUint(models.TagU8, v.Uint())
	case reflect.Uint16:
		return w.walkUint(models.TagU16, v.Uint())
	case reflect.Uint32:
		return w.walkUint(models.TagU32, v.Uint())
	case reflect.Uint, reflect.Uint64:
		// Only above the int64 range: see the Codec documentation.
		if v.Uint() > math.MaxInt64 {
			return w.walkUint(models.TagU64, v.Uint())
		}
		return w.copyItem()
	case reflect.Float32:
		return w.walkFloat32(v.Float())
	case reflect.Slice, reflect.Array:
		return w.walkSequence(v, depth)
	case reflect.Map:
		return w.walkMap(v, depth)
	case reflect.Struct:
		return w.walkStruct(v, depth)
	default:
		return w.copyItem()
	}
}

func (w *sizedWalker) walkInt(tag uint64, want int64) error {
	h, err := w.peekHead()
	if err != nil {
		return err
	}
	var got int64
	switch {
	case h.major == 0 && h.arg <= math.MaxInt64:
		got = int64(h.arg)
	case h.major == 1 && h.arg <= math.MaxInt64:
		got = -1 - int64(h.arg)
	default:
		return w.copyItem()
	}
	if got != want {
		return w.copyItem()
	}
	return w.tagItem(tag)
}

func (w *sizedWalker) walkUint(tag, want uint64) error {
	h, err := w.peekHead()
	if err != nil {
		return err
	}
	if h.major != 0 || h.arg != want {
		return w.copyItem()
	}
	return w.tagItem(tag)
}

// peekFloat returns the float at the current position, if there is one.
func (w *sizedWalker) peekFloat() (float64, bool) {
	if w.pos >= len(w.in) {
		return 0, false
	}
	rest := w.in[w.pos+1:]
	switch w.in[w.pos] {
	case 0xf9:
		if len(rest) < 2 {
			return 0, false
		}
		return float64(float16ToFloat32(binary.BigEndian.Uint16(rest))), true
	case 0xfa:
		if len(rest) < 4 {
			return 0, false
		}
		return float64(math.Float32frombits(binary.BigEndian.Uint32(rest))), true
	case 0xfb:
		if len(rest) < 8 {
			return 0, false
		}
		return math.Float64frombits(binary.BigEndian.Uint64(rest)), true
	}
	return 0, false
}

func sameFloat(a, b float64) bool {
	if math.IsNaN(a) || math.IsNaN(b) {
		return math.IsNaN(a) && math.IsNaN(b)
	}
	return a == b && math.Signbit(a) == math.Signbit(b)
}

func (w *sizedWalker) walkFloat32(want float64) error {
	got, ok := w.peekFloat()
	if !ok || !sameFloat(got, want) {
		return w.copyItem()
	}
	return w.tagItem(models.TagF32)
}

func (w *sizedWalker) walkFloat16(want models.Float16) error {
	got, ok := w.peekFloat()
	if !ok || !sameFloat(got, want.Float64()) {
		return w.copyItem()
	}
	return w.tagItem(models.TagF16)
}

// walkRecordID walks tag 8 [table, key], tagging sized numbers in the key.
// The server reads a sized integer key as a plain int key, and keeps the
// widths inside an array or object key.
func (w *sizedWalker) walkRecordID(v reflect.Value, depth int) error {
	h, err := w.peekHead()
	if err != nil {
		return err
	}
	if h.major != 6 || h.arg != models.TagRecordID {
		return w.copyItem()
	}
	arr, err := readHead(w.in, w.pos+h.size)
	if err != nil {
		return err
	}
	if arr.major != 4 || arr.indefinite || arr.arg != 2 {
		return w.copyItem()
	}
	w.copyHead(h)
	w.copyHead(arr)
	if err := w.copyItem(); err != nil { // the table name
		return err
	}
	return w.walk(v.FieldByName("ID"), depth+1)
}

func (w *sizedWalker) walkSequence(v reflect.Value, depth int) error {
	h, err := w.peekHead()
	if err != nil {
		return err
	}
	if h.major != 4 || h.indefinite || h.arg != uint64(v.Len()) { //nolint:gosec // a length is never negative
		// Byte slices and arrays are byte strings, for one.
		return w.copyItem()
	}
	w.copyHead(h)
	for i := 0; i < v.Len(); i++ {
		if err := w.walk(v.Index(i), depth+1); err != nil {
			return err
		}
	}
	return nil
}

// readText returns the text string at the current position without consuming
// it, if it is a definite-length text string.
func (w *sizedWalker) readText() (string, bool) {
	h, err := w.peekHead()
	if err != nil || h.major != 3 || h.indefinite {
		return "", false
	}
	start := w.pos + h.size
	if h.arg > uint64(len(w.in)-start) { //nolint:gosec // start <= len(w.in)
		return "", false
	}
	return string(w.in[start : start+int(h.arg)]), true //nolint:gosec // bounded by len(w.in) just above
}

func (w *sizedWalker) walkMap(v reflect.Value, depth int) error {
	h, err := w.peekHead()
	if err != nil {
		return err
	}
	if h.major != 5 || h.indefinite || h.arg != uint64(v.Len()) { //nolint:gosec // a length is never negative
		return w.copyItem()
	}
	w.copyHead(h)
	keyType := v.Type().Key()
	for i := uint64(0); i < h.arg; i++ {
		var elem reflect.Value
		if key, ok := w.readText(); ok {
			switch {
			case keyType.Kind() == reflect.String && !encodesItself(keyType):
				elem = v.MapIndex(reflect.ValueOf(key).Convert(keyType))
			case keyType.Kind() == reflect.Interface && reflect.TypeOf(key).Implements(keyType):
				elem = v.MapIndex(reflect.ValueOf(key))
			}
		}
		if err := w.copyItem(); err != nil { // the key
			return err
		}
		if !elem.IsValid() {
			if err := w.copyItem(); err != nil {
				return err
			}
			continue
		}
		if err := w.walk(elem, depth+1); err != nil {
			return err
		}
	}
	return nil
}

func (w *sizedWalker) walkStruct(v reflect.Value, depth int) error {
	h, err := w.peekHead()
	if err != nil {
		return err
	}
	if h.major != 5 || h.indefinite {
		// toarray structs, and anything else unexpected, stay as they are.
		return w.copyItem()
	}
	w.copyHead(h)
	fields := encodedFields(v.Type())
	for i := uint64(0); i < h.arg; i++ {
		var field reflect.Value
		if key, ok := w.readText(); ok {
			if index, ok := fields[key]; ok {
				if f, err := v.FieldByIndexErr(index); err == nil {
					field = f
				}
			}
		}
		if err := w.copyItem(); err != nil { // the key
			return err
		}
		if !field.IsValid() {
			if err := w.copyItem(); err != nil {
				return err
			}
			continue
		}
		if err := w.walk(field, depth+1); err != nil {
			return err
		}
	}
	return nil
}

var encodedFieldsCache sync.Map // reflect.Type -> map[string][]int

// encodedField is a struct field fxamacker/cbor may encode.
type encodedField struct {
	name     string
	index    []int
	tagged   bool // it has a cbor or json tag, so wins a name conflict at its depth
	keyAsInt bool // its name is encoded as an integer
}

// encodedFields maps the names fxamacker/cbor gives a struct's fields when it
// encodes them as text to the field's index path. It follows getFields and
// appendFields in fxamacker/cbor v2.9.2 (encoding/json's visibility rules): the
// cbor tag name, else the json tag name, else the field name, with the fields
// of embedded structs promoted. A name that is ambiguous is left out.
func encodedFields(t reflect.Type) map[string][]int {
	if cached, ok := encodedFieldsCache.Load(t); ok {
		return cached.(map[string][]int)
	}

	flds, next := appendEncodedFields(t, nil, nil, nil)
	visited := map[reflect.Type]bool{t: true} // types embedded at a shallower depth
	for len(next) > 0 {
		var current map[reflect.Type][][]int
		current, next = next, nil
		for ft, indexes := range current {
			// fxamacker ignores every embedded struct whose type is embedded
			// more than once at the same depth, and one embedded at a
			// shallower depth.
			if len(indexes) > 1 || visited[ft] {
				continue
			}
			visited[ft] = true
			flds, next = appendEncodedFields(ft, indexes[0], flds, next)
		}
	}

	// A name is encoded for the shallowest field with it, if that one is
	// alone at its depth or the only one there with a tag.
	byName := map[string][]encodedField{}
	for _, f := range flds {
		if f.keyAsInt {
			continue // its key is an integer, never the text the walk looks up
		}
		byName[f.name] = append(byName[f.name], f)
	}
	fields := make(map[string][]int, len(byName))
	for name, cands := range byName {
		depth := len(cands[0].index)
		for _, c := range cands[1:] {
			depth = min(depth, len(c.index))
		}
		var shallowest, tagged []encodedField
		for _, c := range cands {
			if len(c.index) == depth {
				shallowest = append(shallowest, c)
				if c.tagged {
					tagged = append(tagged, c)
				}
			}
		}
		switch {
		case len(shallowest) == 1:
			fields[name] = shallowest[0].index
		case len(tagged) == 1:
			fields[name] = tagged[0].index
		}
	}
	encodedFieldsCache.Store(t, fields)
	return fields
}

// appendEncodedFields appends the fields of struct type t, at index path index,
// to flds, and the embedded structs whose fields are promoted to next, as
// appendFields in fxamacker/cbor does.
func appendEncodedFields(
	t reflect.Type, index []int, flds []encodedField, next map[reflect.Type][][]int,
) (all []encodedField, embedded map[reflect.Type][][]int) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		ft := f.Type
		for ft.Kind() == reflect.Pointer {
			ft = ft.Elem()
		}
		// An unexported embedded struct can still promote exported fields.
		if !f.IsExported() && (!f.Anonymous || ft.Kind() != reflect.Struct) {
			continue
		}

		// An empty cbor tag falls back to the json tag.
		tag := f.Tag.Get("cbor")
		if tag == "" {
			tag = f.Tag.Get("json")
		}
		if tag == "-" {
			continue
		}
		tagged := tag != ""
		name, options, _ := strings.Cut(tag, ",")
		keyAsInt := false
		for options != "" {
			var option string
			option, options, _ = strings.Cut(options, ",")
			if option == "keyasint" {
				keyAsInt = true
			}
		}

		fieldIndex := append(append([]int(nil), index...), i)
		if f.Anonymous && ft.Kind() == reflect.Struct && name == "" {
			if next == nil {
				next = map[reflect.Type][][]int{}
			}
			next[ft] = append(next[ft], fieldIndex)
			continue
		}
		if name == "" {
			name = f.Name
		}
		flds = append(flds, encodedField{name: name, index: fieldIndex, tagged: tagged, keyAsInt: keyAsInt})
	}
	return flds, next
}
