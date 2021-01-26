package stencode

import (
	"errors"
	"reflect"
	"unsafe"
)

var (
	errRepeatedHasNil = errors.New("proto: repeated field has nil element")
	ErrNil            = errors.New("proto: Marshal called with nil")
)

type Buffer struct {
	buf   []byte // encode/decode byte stream
	index int    // write point

	// pools of basic types to amortize allocation.
	bools   []bool
	uint8s  []uint8
	uint32s []uint32
	uint64s []uint64

	// extra pools, only used with pointer_reflect.go
	int32s   []int32
	int64s   []int64
	float32s []float32
	float64s []float64
}

// NewBuffer allocates a new Buffer and initializes its internal data to
// the contents of the argument slice.
func NewBuffer(e []byte) *Buffer {
	return &Buffer{buf: e}
}

const maxVarintBytes = 10 // maximum length of a varint

// EncodeVarint returns the varint encoding of x.
// This is the format for the
// int32, int64, uint32, uint64, bool, and enum
// protocol buffer types.
// Not used by the package itself, but helpful to clients
// wishing to use the same encoding.
func EncodeVarint(x uint64) []byte {
	var buf [maxVarintBytes]byte
	var n int
	for n = 0; x > 127; n++ {
		buf[n] = 0x80 | uint8(x&0x7F)
		x >>= 7
	}
	buf[n] = uint8(x)
	n++
	return buf[0:n]
}

// EncodeVarint writes a varint-encoded integer to the Buffer.
// This is the format for the
// int32, int64, uint32, uint64, bool, and enum
// protocol buffer types.
func (p *Buffer) EncodeVarint(x uint64) error {
	for x >= 1<<7 {
		p.buf = append(p.buf, uint8(x&0x7f|0x80))
		x >>= 7
	}
	p.buf = append(p.buf, uint8(x))
	return nil
}

// SizeVarint returns the varint encoding size of an integer.
func SizeVarint(x uint64) int {
	return sizeVarint(x)
}

func sizeVarint(x uint64) (n int) {
	for {
		n++
		x >>= 7
		if x == 0 {
			break
		}
	}
	return n
}

// EncodeFixed64 writes a 64-bit integer to the Buffer.
// This is the format for the
// fixed64, sfixed64, and double protocol buffer types.
func (p *Buffer) EncodeFixed64(x uint64) error {
	p.buf = append(p.buf,
		uint8(x),
		uint8(x>>8),
		uint8(x>>16),
		uint8(x>>24),
		uint8(x>>32),
		uint8(x>>40),
		uint8(x>>48),
		uint8(x>>56))
	return nil
}

func sizeFixed64(x uint64) int {
	return 8
}

// EncodeFixed32 writes a 32-bit integer to the Buffer.
// This is the format for the
// fixed32, sfixed32, and float protocol buffer types.
func (p *Buffer) EncodeFixed32(x uint64) error {
	p.buf = append(p.buf,
		uint8(x),
		uint8(x>>8),
		uint8(x>>16),
		uint8(x>>24))
	return nil
}

func sizeFixed32(x uint64) int {
	return 4
}

// EncodeZigzag64 writes a zigzag-encoded 64-bit integer
// to the Buffer.
// This is the format used for the sint64 protocol buffer type.
func (p *Buffer) EncodeZigzag64(x uint64) error {
	// use signed number to get arithmetic right shift.
	return p.EncodeVarint(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}

func sizeZigzag64(x uint64) int {
	return sizeVarint(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}

// EncodeZigzag32 writes a zigzag-encoded 32-bit integer
// to the Buffer.
// This is the format used for the sint32 protocol buffer type.
func (p *Buffer) EncodeZigzag32(x uint64) error {
	// use signed number to get arithmetic right shift.
	return p.EncodeVarint(uint64((uint32(x) << 1) ^ uint32((int32(x) >> 31))))
}

func sizeZigzag32(x uint64) int {
	return sizeVarint(uint64((uint32(x) << 1) ^ uint32((int32(x) >> 31))))
}

// EncodeRawBytes writes a count-delimited byte buffer to the Buffer.
// This is the format used for the bytes protocol buffer
// type and for embedded messages.
func (p *Buffer) EncodeRawBytes(b []byte) error {
	p.EncodeVarint(uint64(len(b)))
	p.buf = append(p.buf, b...)
	return nil
}

func sizeRawBytes(b []byte) int {
	return sizeVarint(uint64(len(b))) +
		len(b)
}

// EncodeStringBytes writes an encoded string to the Buffer.
// This is the format used for the proto2 string type.
func (p *Buffer) EncodeStringBytes(s string) error {
	p.EncodeVarint(uint64(len(s)))
	p.buf = append(p.buf, s...)
	return nil
}

func sizeStringBytes(s string) int {
	return sizeVarint(uint64(len(s))) +
		len(s)
}

// Marshal takes the protocol buffer
// and encodes it into the wire format, returning the data.
func Marshal(pb interface{}) ([]byte, error) {
	p := NewBuffer(nil)
	err := p.Marshal(pb)
	if p.buf == nil && err == nil {
		// Return a non-nil slice on success.
		return []byte{}, nil
	}
	return p.buf, err
}

// Marshal takes the protocol buffer
// and encodes it into the wire format, writing the result to the
// Buffer.
func (p *Buffer) Marshal(pb interface{}) error {
	t := reflect.TypeOf(pb)
	base := structPointer(reflect.ValueOf(pb).Pointer())
	if base == nil {
		return errors.New("Marshal called with nil")
	}
	err := p.enc_struct(GetProperties(t.Elem()), base)
	return err
}

var zeroes [20]byte // longer than any conceivable sizeVarint

// Encode a struct, preceded by its encoded length (as a varint).
func (o *Buffer) enc_len_struct(prop *StructProperties, base structPointer) error {
	return o.enc_len_thing(func() error { return o.enc_struct(prop, base) })
}

// Encode something, preceded by its encoded length (as a varint).
func (o *Buffer) enc_len_thing(enc func() error) error {
	iLen := len(o.buf)
	o.buf = append(o.buf, 0, 0, 0, 0) // reserve four bytes for length
	iMsg := len(o.buf)
	err := enc()
	if err != nil {
		return err
	}
	lMsg := len(o.buf) - iMsg
	lLen := sizeVarint(uint64(lMsg))
	switch x := lLen - (iMsg - iLen); {
	case x > 0: // actual length is x bytes larger than the space we reserved
		// Move msg x bytes right.
		o.buf = append(o.buf, zeroes[:x]...)
		copy(o.buf[iMsg+x:], o.buf[iMsg:iMsg+lMsg])
	case x < 0: // actual length is x bytes smaller than the space we reserved
		// Move msg x bytes left.
		copy(o.buf[iMsg+x:], o.buf[iMsg:iMsg+lMsg])
		o.buf = o.buf[:len(o.buf)+x] // x is negative
	}
	// Encode the length in the reserved space.
	o.buf = o.buf[:iLen]
	o.EncodeVarint(uint64(lMsg))
	o.buf = o.buf[:len(o.buf)+lMsg]
	return err
}

// Encode a struct.
func (o *Buffer) enc_struct(prop *StructProperties, base structPointer) error {
	// Encode fields in tag order so that decoders may use optimizations
	// that depend on the ordering.
	// https://developers.google.com/protocol-buffers/docs/encoding#order
	for _, i := range prop.order {
		p := prop.Prop[i]
		if p.enc != nil {
			err := p.enc(o, p, base)
			if err != nil {
				if err == ErrNil {
				} else if err == errRepeatedHasNil {
					// Give more context to nil values in repeated fields.
					return errors.New("repeated field " + p.OrigName + " has nil element")
				}
			}
		}
	}

	return nil
}

func size_struct(prop *StructProperties, base structPointer) (n int) {
	for _, i := range prop.order {
		p := prop.Prop[i]
		if p.size != nil {
			n += p.size(p, base)
		}
	}
	return
}

// Encode a bool.
func (o *Buffer) enc_bool(p *Properties, base structPointer) error {
	var v bool
	if p.isPtr {
		v = **(**bool)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	} else {
		v = *(*bool)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	}

	if !v {
		return ErrNil
	}
	o.buf = append(o.buf, p.tagcode...)
	p.valEnc(o, 1)
	return nil
}

func size_bool(p *Properties, base structPointer) int {
	var v bool
	if p.isPtr {
		v = **(**bool)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	} else {
		v = *(*bool)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	}
	if !v {
		return 0
	}
	return len(p.tagcode) + 1 // each bool takes exactly one byte
}

// Encode an uint8.
func (o *Buffer) enc_uint8(p *Properties, base structPointer) error {
	var v uint8
	if p.isPtr {
		v = **(**uint8)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	} else {
		v = *(*uint8)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	}

	x := v
	if x == 0 {
		return ErrNil
	}
	o.buf = append(o.buf, p.tagcode...)
	p.valEnc(o, uint64(x))
	return nil
}

func size_uint8(p *Properties, base structPointer) (n int) {
	var v uint8
	if p.isPtr {
		v = **(**uint8)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	} else {
		v = *(*uint8)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	}

	x := v
	if x == 0 {
		return 0
	}
	n += len(p.tagcode)
	n += p.valSize(uint64(x))
	return
}

// Encode an int.
func (o *Buffer) enc_int(p *Properties, base structPointer) error {
	var v int
	if p.isPtr {
		v = **(**int)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	} else {
		v = *(*int)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	}

	x := v
	if x == 0 {
		return ErrNil
	}
	o.buf = append(o.buf, p.tagcode...)
	p.valEnc(o, uint64(x))
	return nil
}

func size_int(p *Properties, base structPointer) (n int) {
	var v int
	if p.isPtr {
		v = **(**int)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	} else {
		v = *(*int)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	}

	x := v
	if x == 0 {
		return 0
	}
	n += len(p.tagcode)
	n += p.valSize(uint64(x))
	return
}

// Encode an int32.
func (o *Buffer) enc_int32(p *Properties, base structPointer) error {
	var v int32
	if p.isPtr {
		v = **(**int32)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	} else {
		v = *(*int32)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	}

	x := v
	if x == 0 {
		return ErrNil
	}
	o.buf = append(o.buf, p.tagcode...)
	p.valEnc(o, uint64(x))
	return nil
}

func size_int32(p *Properties, base structPointer) (n int) {
	var v int32
	if p.isPtr {
		v = **(**int32)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	} else {
		v = *(*int32)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	}

	x := v
	if x == 0 {
		return 0
	}
	n += len(p.tagcode)
	n += p.valSize(uint64(x))
	return
}

// Encode a uint32.
// Exactly the same as int32, except for no sign extension.
func (o *Buffer) enc_uint32(p *Properties, base structPointer) error {
	var v uint32
	if p.isPtr {
		v = **(**uint32)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	} else {
		v = *(*uint32)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	}

	x := v
	if x == 0 {
		return ErrNil
	}
	o.buf = append(o.buf, p.tagcode...)
	p.valEnc(o, uint64(x))
	return nil
}

func size_uint32(p *Properties, base structPointer) (n int) {
	var v uint32
	if p.isPtr {
		v = **(**uint32)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	} else {
		v = *(*uint32)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	}

	x := v
	if x == 0 {
		return 0
	}
	n += len(p.tagcode)
	n += p.valSize(uint64(x))
	return
}

// Encode an int64.
func (o *Buffer) enc_int64(p *Properties, base structPointer) error {
	var v uint64
	if p.isPtr {
		v = **(**uint64)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	} else {
		v = *(*uint64)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	}

	x := v
	if x == 0 {
		return ErrNil
	}
	o.buf = append(o.buf, p.tagcode...)
	p.valEnc(o, x)
	return nil
}

func size_int64(p *Properties, base structPointer) (n int) {
	var v uint64
	if p.isPtr {
		v = **(**uint64)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	} else {
		v = *(*uint64)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	}

	x := v
	if x == 0 {
		return 0
	}
	n += len(p.tagcode)
	n += p.valSize(x)
	return
}

// Encode a string.
func (o *Buffer) enc_string(p *Properties, base structPointer) error {
	var v string
	if p.isPtr {
		v = **(**string)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	} else {
		v = *(*string)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	}

	if v == "" {
		return ErrNil
	}
	o.buf = append(o.buf, p.tagcode...)
	o.EncodeStringBytes(v)
	return nil
}

func size_string(p *Properties, base structPointer) (n int) {
	var v string
	if p.isPtr {
		v = **(**string)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	} else {
		v = *(*string)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	}

	if v == "" {
		return 0
	}
	n += len(p.tagcode)
	n += sizeStringBytes(v)
	return
}

// All protocol buffer fields are nillable, but be careful.
func isNil(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return v.IsNil()
	}
	return false
}

// Encode a message struct.
func (o *Buffer) enc_struct_message(p *Properties, base structPointer) error {
	var structp structPointer
	if p.isPtr {
		structp = *(*structPointer)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	} else {
		structp = (structPointer)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	}
	if structp == nil {
		return ErrNil
	}

	o.buf = append(o.buf, p.tagcode...)
	return o.enc_len_struct(p.sprop, structp)
}

func size_struct_message(p *Properties, base structPointer) int {
	var structp structPointer
	if p.isPtr {
		structp = *(*structPointer)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	} else {
		structp = (structPointer)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	}
	if structp == nil {
		return 0
	}

	n0 := len(p.tagcode)
	n1 := size_struct(p.sprop, structp)
	n2 := sizeVarint(uint64(n1)) // size of encoded length
	return n0 + n1 + n2
}

// Encode a slice of bools ([]bool).
func (o *Buffer) enc_slice_bool(p *Properties, base structPointer) error {
	s := *(*[]bool)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	l := len(s)
	if l == 0 {
		return ErrNil
	}
	for _, x := range s {
		o.buf = append(o.buf, p.tagcode...)
		v := uint64(0)
		if x {
			v = 1
		}
		p.valEnc(o, v)
	}
	return nil
}

func size_slice_bool(p *Properties, base structPointer) int {
	s := *(*[]bool)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	l := len(s)
	if l == 0 {
		return 0
	}
	return l * (len(p.tagcode) + 1) // each bool takes exactly one byte
}

// Encode a slice of bytes ([]byte).
func (o *Buffer) enc_slice_byte(p *Properties, base structPointer) error {
	s := *(*[]byte)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	if len(s) == 0 {
		return ErrNil
	}
	o.buf = append(o.buf, p.tagcode...)
	o.EncodeRawBytes(s)
	return nil
}

func size_slice_byte(p *Properties, base structPointer) (n int) {
	s := *(*[]byte)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	if len(s) == 0 {
		return 0
	}
	n += len(p.tagcode)
	n += sizeRawBytes(s)
	return
}

// Encode a slice of int32s ([]int32).
func (o *Buffer) enc_slice_int32(p *Properties, base structPointer) error {
	s := *(*[]uint32)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	l := len(s)
	if l == 0 {
		return ErrNil
	}
	for i := 0; i < l; i++ {
		o.buf = append(o.buf, p.tagcode...)
		x := int32(s[i]) // permit sign extension to use full 64-bit range
		p.valEnc(o, uint64(x))
	}
	return nil
}

func size_slice_int32(p *Properties, base structPointer) (n int) {
	s := *(*[]uint32)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	l := len(s)
	if l == 0 {
		return 0
	}
	for i := 0; i < l; i++ {
		n += len(p.tagcode)
		x := int32(s[i]) // permit sign extension to use full 64-bit range
		n += p.valSize(uint64(x))
	}
	return
}

// Encode a slice of uint32s ([]uint32).
// Exactly the same as int32, except for no sign extension.
func (o *Buffer) enc_slice_uint32(p *Properties, base structPointer) error {
	s := *(*[]uint32)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	l := len(s)
	if l == 0 {
		return ErrNil
	}
	for i := 0; i < l; i++ {
		o.buf = append(o.buf, p.tagcode...)
		x := s[i]
		p.valEnc(o, uint64(x))
	}
	return nil
}

func size_slice_uint32(p *Properties, base structPointer) (n int) {
	s := *(*[]uint32)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	l := len(s)
	if l == 0 {
		return 0
	}
	for i := 0; i < l; i++ {
		n += len(p.tagcode)
		x := s[i]
		n += p.valSize(uint64(x))
	}
	return
}

// Encode a slice of int64s ([]int64).
func (o *Buffer) enc_slice_int64(p *Properties, base structPointer) error {
	s := *(*[]uint64)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	l := len(s)
	if l == 0 {
		return ErrNil
	}
	for i := 0; i < l; i++ {
		o.buf = append(o.buf, p.tagcode...)
		p.valEnc(o, s[i])
	}
	return nil
}

func size_slice_int64(p *Properties, base structPointer) (n int) {
	s := *(*[]uint64)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	l := len(s)
	if l == 0 {
		return 0
	}
	for i := 0; i < l; i++ {
		n += len(p.tagcode)
		n += p.valSize(s[i])
	}
	return
}

// Encode a slice of slice of bytes ([][]byte).
func (o *Buffer) enc_slice_slice_byte(p *Properties, base structPointer) error {
	ss := *(*[][]byte)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	l := len(ss)
	if l == 0 {
		return ErrNil
	}
	for i := 0; i < l; i++ {
		o.buf = append(o.buf, p.tagcode...)
		o.EncodeRawBytes(ss[i])
	}
	return nil
}

func size_slice_slice_byte(p *Properties, base structPointer) (n int) {
	ss := *(*[][]byte)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	l := len(ss)
	if l == 0 {
		return 0
	}
	n += l * len(p.tagcode)
	for i := 0; i < l; i++ {
		n += sizeRawBytes(ss[i])
	}
	return
}

// Encode a slice of strings ([]string).
func (o *Buffer) enc_slice_string(p *Properties, base structPointer) error {
	ss := *(*[]string)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	l := len(ss)
	for i := 0; i < l; i++ {
		o.buf = append(o.buf, p.tagcode...)
		o.EncodeStringBytes(ss[i])
	}
	return nil
}

func size_slice_string(p *Properties, base structPointer) (n int) {
	ss := *(*[]string)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	l := len(ss)
	n += l * len(p.tagcode)
	for i := 0; i < l; i++ {
		n += sizeStringBytes(ss[i])
	}
	return
}

type structPointerSlice []structPointer

func (v *structPointerSlice) Len() int                  { return len(*v) }
func (v *structPointerSlice) Index(i int) structPointer { return (*v)[i] }
func (v *structPointerSlice) Append(p structPointer)    { *v = append(*v, p) }

// Encode a slice of message structs ([]*struct).
func (o *Buffer) enc_slice_struct_message(p *Properties, base structPointer) error {
	s := (*structPointerSlice)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	l := s.Len()

	for i := 0; i < l; i++ {
		structp := s.Index(i)
		if structp == nil {
			return errRepeatedHasNil
		}

		o.buf = append(o.buf, p.tagcode...)
		err := o.enc_len_struct(p.sprop, structp)
		if err != nil {
			if err == ErrNil {
				return errRepeatedHasNil
			}
			return err
		}
	}
	return nil
}

func size_slice_struct_message(p *Properties, base structPointer) (n int) {
	s := (*structPointerSlice)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	l := s.Len()
	n += l * len(p.tagcode)
	for i := 0; i < l; i++ {
		structp := s.Index(i)
		if structp == nil {
			return // return the size up to this point
		}

		n0 := size_struct(p.sprop, structp)
		n1 := sizeVarint(uint64(n0)) // size of encoded length
		n += n0 + n1
	}
	return
}

// Encode a slice of message structs ([]struct).
func (o *Buffer) enc_slice_struct_message_s(p *Properties, base structPointer) error {
	s := structPointer_NewAt(base, p.field, p.mtype).Elem() // slice
	l := s.Len()

	for i := 0; i < l; i++ {
		structp := structPointer(s.Index(i).UnsafeAddr())
		if structp == nil {
			return errRepeatedHasNil
		}

		o.buf = append(o.buf, p.tagcode...)
		err := o.enc_len_struct(p.sprop, structp)
		if err != nil {
			if err == ErrNil {
				return errRepeatedHasNil
			}
			return err
		}
	}
	return nil
}

func size_slice_struct_message_s(p *Properties, base structPointer) (n int) {
	s := structPointer_NewAt(base, p.field, p.mtype).Elem() // slice
	l := s.Len()
	n += l * len(p.tagcode)
	for i := 0; i < l; i++ {
		structp := structPointer(s.Index(i).UnsafeAddr())
		if structp == nil {
			return // return the size up to this point
		}

		n0 := size_struct(p.sprop, structp)
		n1 := sizeVarint(uint64(n0)) // size of encoded length
		n += n0 + n1
	}
	return
}

// Encode a map field.
func (o *Buffer) enc_new_map(p *Properties, base structPointer) error {

	/*
		A map defined as
			map<key_type, value_type> map_field = N;
		is encoded in the same way as
			message MapFieldEntry {
				key_type key = 1;
				value_type value = 2;
			}
			repeated MapFieldEntry map_field = N;
	*/

	v := structPointer_NewAt(base, p.field, p.mtype).Elem() // map[K]V
	if v.Len() == 0 {
		return nil
	}

	keycopy, valcopy, keybase, valbase := mapEncodeScratch(p.mtype)

	enc := func() error {
		if err := p.mkeyprop.enc(o, p.mkeyprop, keybase); err != nil {
			return err
		}
		if err := p.mvalprop.enc(o, p.mvalprop, valbase); err != nil && err != ErrNil {
			return err
		}
		return nil
	}

	// Don't sort map keys. It is not required by the spec, and C++ doesn't do it.
	for _, key := range v.MapKeys() {
		val := v.MapIndex(key)

		keycopy.Set(key)
		valcopy.Set(val)

		o.buf = append(o.buf, p.tagcode...)
		if err := o.enc_len_thing(enc); err != nil {
			return err
		}
	}
	return nil
}

func size_new_map(p *Properties, base structPointer) int {
	v := structPointer_NewAt(base, p.field, p.mtype).Elem() // map[K]V

	keycopy, valcopy, keybase, valbase := mapEncodeScratch(p.mtype)

	n := 0
	for _, key := range v.MapKeys() {
		val := v.MapIndex(key)
		keycopy.Set(key)
		valcopy.Set(val)

		// Tag codes for key and val are the responsibility of the sub-sizer.
		keysize := p.mkeyprop.size(p.mkeyprop, keybase)
		valsize := p.mvalprop.size(p.mvalprop, valbase)
		entry := keysize + valsize
		// Add on tag code and length of map entry itself.
		n += len(p.tagcode) + sizeVarint(uint64(entry)) + entry
	}
	return n
}

// mapEncodeScratch returns a new reflect.Value matching the map's value type,
// and a structPointer suitable for passing to an encoder or sizer.
func mapEncodeScratch(mapType reflect.Type) (keycopy, valcopy reflect.Value, keybase, valbase structPointer) {
	// Prepare addressable doubly-indirect placeholders for the key and value types.
	// This is needed because the element-type encoders expect **T, but the map iteration produces T.

	keycopy = reflect.New(mapType.Key()).Elem()                 // addressable K
	keyptr := reflect.New(reflect.PtrTo(keycopy.Type())).Elem() // addressable *K
	keyptr.Set(keycopy.Addr())                                  //
	keybase = toStructPointer(keyptr.Addr())                    // **K

	// Value types are more varied and require special handling.
	switch mapType.Elem().Kind() {
	case reflect.Slice:
		// []byte
		var dummy []byte
		valcopy = reflect.ValueOf(&dummy).Elem() // addressable []byte
		valbase = toStructPointer(valcopy.Addr())
	case reflect.Ptr:
		// message; the generated field type is map[K]*Msg (so V is *Msg),
		// so we only need one level of indirection.
		valcopy = reflect.New(mapType.Elem()).Elem() // addressable V
		valbase = toStructPointer(valcopy.Addr())
	default:
		// everything else
		valcopy = reflect.New(mapType.Elem()).Elem()                // addressable V
		valptr := reflect.New(reflect.PtrTo(valcopy.Type())).Elem() // addressable *V
		valptr.Set(valcopy.Addr())                                  //
		valbase = toStructPointer(valptr.Addr())                    // **V
	}
	return
}
