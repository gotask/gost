package stencode

import (
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"unsafe"
)

// errOverflow is returned when an integer is too large to be represented.
var errOverflow = errors.New("proto: integer overflow")

// DecodeVarint reads a varint-encoded integer from the slice.
// It returns the integer and the number of bytes consumed, or
// zero if there is not enough.
// This is the format for the
// int32, int64, uint32, uint64, bool, and enum
// protocol buffer types.
func DecodeVarint(buf []byte) (x uint64, n int) {
	// x, n already 0
	for shift := uint(0); shift < 64; shift += 7 {
		if n >= len(buf) {
			return 0, 0
		}
		b := uint64(buf[n])
		n++
		x |= (b & 0x7F) << shift
		if (b & 0x80) == 0 {
			return x, n
		}
	}

	// The number is too large to represent in a 64-bit value.
	return 0, 0
}

// DecodeVarint reads a varint-encoded integer from the Buffer.
// This is the format for the
// int32, int64, uint32, uint64, bool, and enum
// protocol buffer types.
func (p *Buffer) DecodeVarint() (x uint64, err error) {
	// x, err already 0

	i := p.index
	l := len(p.buf)

	for shift := uint(0); shift < 64; shift += 7 {
		if i >= l {
			err = io.ErrUnexpectedEOF
			return
		}
		b := p.buf[i]
		i++
		x |= (uint64(b) & 0x7F) << shift
		if b < 0x80 {
			p.index = i
			return
		}
	}

	// The number is too large to represent in a 64-bit value.
	err = errOverflow
	return
}

// DecodeFixed64 reads a 64-bit integer from the Buffer.
// This is the format for the
// fixed64, sfixed64, and double protocol buffer types.
func (p *Buffer) DecodeFixed64() (x uint64, err error) {
	// x, err already 0
	i := p.index + 8
	if i < 0 || i > len(p.buf) {
		err = io.ErrUnexpectedEOF
		return
	}
	p.index = i

	x = uint64(p.buf[i-8])
	x |= uint64(p.buf[i-7]) << 8
	x |= uint64(p.buf[i-6]) << 16
	x |= uint64(p.buf[i-5]) << 24
	x |= uint64(p.buf[i-4]) << 32
	x |= uint64(p.buf[i-3]) << 40
	x |= uint64(p.buf[i-2]) << 48
	x |= uint64(p.buf[i-1]) << 56
	return
}

// DecodeFixed32 reads a 32-bit integer from the Buffer.
// This is the format for the
// fixed32, sfixed32, and float protocol buffer types.
func (p *Buffer) DecodeFixed32() (x uint64, err error) {
	// x, err already 0
	i := p.index + 4
	if i < 0 || i > len(p.buf) {
		err = io.ErrUnexpectedEOF
		return
	}
	p.index = i

	x = uint64(p.buf[i-4])
	x |= uint64(p.buf[i-3]) << 8
	x |= uint64(p.buf[i-2]) << 16
	x |= uint64(p.buf[i-1]) << 24
	return
}

// DecodeZigzag64 reads a zigzag-encoded 64-bit integer
// from the Buffer.
// This is the format used for the sint64 protocol buffer type.
func (p *Buffer) DecodeZigzag64() (x uint64, err error) {
	x, err = p.DecodeVarint()
	if err != nil {
		return
	}
	x = (x >> 1) ^ uint64((int64(x&1)<<63)>>63)
	return
}

// DecodeZigzag32 reads a zigzag-encoded 32-bit integer
// from  the Buffer.
// This is the format used for the sint32 protocol buffer type.
func (p *Buffer) DecodeZigzag32() (x uint64, err error) {
	x, err = p.DecodeVarint()
	if err != nil {
		return
	}
	x = uint64((uint32(x) >> 1) ^ uint32((int32(x&1)<<31)>>31))
	return
}

// These are not ValueDecoders: they produce an array of bytes or a string.
// bytes, embedded messages

// DecodeRawBytes reads a count-delimited byte buffer from the Buffer.
// This is the format used for the bytes protocol buffer
// type and for embedded messages.
func (p *Buffer) DecodeRawBytes(alloc bool) (buf []byte, err error) {
	n, err := p.DecodeVarint()
	if err != nil {
		return nil, err
	}

	nb := int(n)
	if nb < 0 {
		return nil, fmt.Errorf("proto: bad byte length %d", nb)
	}
	end := p.index + nb
	if end < p.index || end > len(p.buf) {
		return nil, io.ErrUnexpectedEOF
	}

	if !alloc {
		// todo: check if can get more uses of alloc=false
		buf = p.buf[p.index:end]
		p.index += nb
		return
	}

	buf = make([]byte, nb)
	copy(buf, p.buf[p.index:])
	p.index += nb
	return
}

// DecodeStringBytes reads an encoded string from the Buffer.
// This is the format used for the proto2 string type.
func (p *Buffer) DecodeStringBytes() (s string, err error) {
	buf, err := p.DecodeRawBytes(false)
	if err != nil {
		return
	}
	return string(buf), nil
}

// DecodeMessage reads a count-delimited message from the Buffer.
func (p *Buffer) DecodeMessage(pb interface{}) error {
	enc, err := p.DecodeRawBytes(false)
	if err != nil {
		return err
	}
	return NewBuffer(enc).Unmarshal(pb)
}

// DecodeGroup reads a tag-delimited group from the Buffer.
func (p *Buffer) DecodeGroup(pb interface{}) error {
	t := reflect.TypeOf(pb)
	base := structPointer(reflect.ValueOf(pb).Pointer())
	if base == nil {
		return errors.New("Marshal called with nil")
	}
	return p.unmarshalType(t.Elem(), GetProperties(t.Elem()), true, base)
}

func Unmarshal(buf []byte, pb interface{}) error {
	return NewBuffer(buf).Unmarshal(pb)
}

// Unmarshal parses the protocol buffer representation in the
// Buffer and places the decoded result in pb.  If the struct
// underlying pb does not match the data in the buffer, the results can be
// unpredictable.
func (p *Buffer) Unmarshal(pb interface{}) error {
	t := reflect.TypeOf(pb)
	base := structPointer(reflect.ValueOf(pb).Pointer())
	if base == nil {
		return errors.New("Marshal called with nil")
	}

	err := p.unmarshalType(t.Elem(), GetProperties(t.Elem()), false, base)

	return err
}

// unmarshalType does the work of unmarshaling a structure.
func (o *Buffer) unmarshalType(st reflect.Type, prop *StructProperties, is_group bool, base structPointer) error {
	var err error
	for err == nil && o.index < len(o.buf) {
		var u uint64
		u, err = o.DecodeVarint()
		if err != nil {
			break
		}
		wire := int(u & 0x7)
		if wire == WireEndGroup {
			if is_group {
				return nil // input is satisfied
			}
			return fmt.Errorf("proto: %s: wiretype end group for non-group", st)
		}
		tag := int(u >> 3)
		if tag <= 0 {
			return fmt.Errorf("proto: %s: illegal tag %d (wire type %d)", st, tag, wire)
		}
		fieldnum := tag - 1
		if fieldnum < 0 || fieldnum >= len(prop.Prop) {
			err = o.skip(st, tag, wire)
			continue
		}
		p := prop.Prop[fieldnum]

		if p.dec == nil {
			fmt.Fprintf(os.Stderr, "proto: no protobuf decoder for %s.%s\n", st, st.Field(fieldnum).Name)
			continue
		}
		dec := p.dec
		decErr := dec(o, p, base)
		if decErr != nil {
			err = decErr
		}
	}
	if err == nil {
		if is_group {
			return io.ErrUnexpectedEOF
		}
	}
	return err
}

// Skip the next item in the buffer. Its wire type is decoded and presented as an argument.
func (o *Buffer) skip(t reflect.Type, tag, wire int) error {

	var u uint64
	var err error

	switch wire {
	case WireVarint:
		_, err = o.DecodeVarint()
	case WireFixed64:
		_, err = o.DecodeFixed64()
	case WireBytes:
		_, err = o.DecodeRawBytes(false)
	case WireFixed32:
		_, err = o.DecodeFixed32()
	case WireStartGroup:
		for {
			u, err = o.DecodeVarint()
			if err != nil {
				break
			}
			fwire := int(u & 0x7)
			if fwire == WireEndGroup {
				break
			}
			ftag := int(u >> 3)
			err = o.skip(t, ftag, fwire)
			if err != nil {
				break
			}
		}
	default:
		err = fmt.Errorf("proto: can't skip unknown wire type %d for %s", wire, t)
	}
	return err
}

// Individual type decoders
// For each,
//	u is the decoded value,
//	v is a pointer to the field (pointer) in the struct

// Sizes of the pools to allocate inside the Buffer.
// The goal is modest amortization and allocation
// on at least 16-byte boundaries.
const (
	boolPoolSize   = 16
	uint32PoolSize = 8
	uint64PoolSize = 4
)

// Decode a bool.
func (o *Buffer) dec_bool(p *Properties, base structPointer) error {
	u, err := p.valDec(o)
	if err != nil {
		return err
	}
	if p.isPtr {
		if len(o.bools) == 0 {
			o.bools = make([]bool, boolPoolSize)
		}
		o.bools[0] = u != 0
		*(**bool)(unsafe.Pointer(uintptr(base) + uintptr(p.field))) = &o.bools[0]
		o.bools = o.bools[1:]
	} else {
		*(*bool)(unsafe.Pointer(uintptr(base) + uintptr(p.field))) = u != 0
	}

	return nil
}

// Decode an uint8.
func (o *Buffer) dec_uint8(p *Properties, base structPointer) error {
	u, err := p.valDec(o)
	if err != nil {
		return err
	}

	if p.isPtr {
		if len(o.uint8s) == 0 {
			o.uint8s = make([]uint8, boolPoolSize)
		}
		o.uint8s[0] = uint8(u)
		*(**uint8)(unsafe.Pointer(uintptr(base) + uintptr(p.field))) = &o.uint8s[0]
		o.uint8s = o.uint8s[1:]
	} else {
		*(*uint8)(unsafe.Pointer(uintptr(base) + uintptr(p.field))) = uint8(u)
	}
	return nil
}

// Decode an int.
func (o *Buffer) dec_int(p *Properties, base structPointer) error {
	if strconv.IntSize == 32 {
		return o.dec_int32(p, base)
	}
	return o.dec_int64(p, base)
}

// Decode an int32.
func (o *Buffer) dec_int32(p *Properties, base structPointer) error {
	u, err := p.valDec(o)
	if err != nil {
		return err
	}

	if p.isPtr {
		if len(o.uint32s) == 0 {
			o.uint32s = make([]uint32, uint32PoolSize)
		}
		o.uint32s[0] = uint32(u)
		*(**uint32)(unsafe.Pointer(uintptr(base) + uintptr(p.field))) = &o.uint32s[0]
		o.uint32s = o.uint32s[1:]
	} else {
		*(*uint32)(unsafe.Pointer(uintptr(base) + uintptr(p.field))) = uint32(u)
	}
	return nil
}

// Decode an int64.
func (o *Buffer) dec_int64(p *Properties, base structPointer) error {
	u, err := p.valDec(o)
	if err != nil {
		return err
	}
	if p.isPtr {
		if len(o.uint64s) == 0 {
			o.uint64s = make([]uint64, uint64PoolSize)
		}
		o.uint64s[0] = u
		*(**uint64)(unsafe.Pointer(uintptr(base) + uintptr(p.field))) = &o.uint64s[0]
		o.uint64s = o.uint64s[1:]
	} else {
		*(*uint64)(unsafe.Pointer(uintptr(base) + uintptr(p.field))) = u
	}
	return nil
}

// Decode a string.
func (o *Buffer) dec_string(p *Properties, base structPointer) error {
	s, err := o.DecodeStringBytes()
	if err != nil {
		return err
	}
	if p.isPtr {
		*(**string)(unsafe.Pointer(uintptr(base) + uintptr(p.field))) = &s
	} else {
		*(*string)(unsafe.Pointer(uintptr(base) + uintptr(p.field))) = s
	}
	return nil
}

// Decode a slice of bytes ([]byte).
func (o *Buffer) dec_slice_byte(p *Properties, base structPointer) error {
	b, err := o.DecodeRawBytes(true)
	if err != nil {
		return err
	}
	*(*[]byte)(unsafe.Pointer(uintptr(base) + uintptr(p.field))) = b
	return nil
}

// Decode a slice of bools ([]bool).
func (o *Buffer) dec_slice_bool(p *Properties, base structPointer) error {
	u, err := p.valDec(o)
	if err != nil {
		return err
	}
	v := (*[]bool)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	*v = append(*v, u != 0)
	return nil
}

// Decode a slice of int32s ([]int32).
func (o *Buffer) dec_slice_int32(p *Properties, base structPointer) error {
	u, err := p.valDec(o)
	if err != nil {
		return err
	}

	v := (*[]uint32)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	*v = append(*v, uint32(u))
	return nil
}

// Decode a slice of int64s ([]int64).
func (o *Buffer) dec_slice_int64(p *Properties, base structPointer) error {
	u, err := p.valDec(o)
	if err != nil {
		return err
	}

	v := (*[]uint64)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	*v = append(*v, uint64(u))
	return nil
}

// Decode a slice of strings ([]string).
func (o *Buffer) dec_slice_string(p *Properties, base structPointer) error {
	s, err := o.DecodeStringBytes()
	if err != nil {
		return err
	}
	v := (*[]string)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	*v = append(*v, s)
	return nil
}

// Decode a slice of slice of bytes ([][]byte).
func (o *Buffer) dec_slice_slice_byte(p *Properties, base structPointer) error {
	b, err := o.DecodeRawBytes(true)
	if err != nil {
		return err
	}
	v := (*[][]byte)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	*v = append(*v, b)
	return nil
}

// Decode a map field.
func (o *Buffer) dec_new_map(p *Properties, base structPointer) error {
	raw, err := o.DecodeRawBytes(false)
	if err != nil {
		return err
	}
	oi := o.index       // index at the end of this map entry
	o.index -= len(raw) // move buffer back to start of map entry

	mptr := structPointer_NewAt(base, p.field, p.mtype) // *map[K]V
	if mptr.Elem().IsNil() {
		mptr.Elem().Set(reflect.MakeMap(mptr.Type().Elem()))
	}
	v := mptr.Elem() // map[K]V

	// Prepare addressable doubly-indirect placeholders for the key and value types.
	// See enc_new_map for why.
	keyptr := reflect.New(reflect.PtrTo(p.mtype.Key())).Elem() // addressable *K
	keybase := toStructPointer(keyptr.Addr())                  // **K

	var valbase structPointer
	var valptr reflect.Value
	switch p.mtype.Elem().Kind() {
	case reflect.Slice:
		// []byte
		var dummy []byte
		valptr = reflect.ValueOf(&dummy)  // *[]byte
		valbase = toStructPointer(valptr) // *[]byte
	case reflect.Ptr:
		// message; valptr is **Msg; need to allocate the intermediate pointer
		valptr = reflect.New(reflect.PtrTo(p.mtype.Elem())).Elem() // addressable *V
		valptr.Set(reflect.New(valptr.Type().Elem()))
		valbase = toStructPointer(valptr)
	default:
		// everything else
		valptr = reflect.New(reflect.PtrTo(p.mtype.Elem())).Elem() // addressable *V
		valbase = toStructPointer(valptr.Addr())                   // **V
	}

	// Decode.
	// This parses a restricted wire format, namely the encoding of a message
	// with two fields. See enc_new_map for the format.
	for o.index < oi {
		// tagcode for key and value properties are always a single byte
		// because they have tags 1 and 2.
		tagcode := o.buf[o.index]
		o.index++
		switch tagcode {
		case p.mkeyprop.tagcode[0]:
			if err := p.mkeyprop.dec(o, p.mkeyprop, keybase); err != nil {
				return err
			}
		case p.mvalprop.tagcode[0]:
			if err := p.mvalprop.dec(o, p.mvalprop, valbase); err != nil {
				return err
			}
		default:
			// TODO: Should we silently skip this instead?
			return fmt.Errorf("proto: bad map data tag %d", raw[0])
		}
	}
	keyelem, valelem := keyptr.Elem(), valptr.Elem()
	if !keyelem.IsValid() {
		keyelem = reflect.Zero(p.mtype.Key())
	}
	if !valelem.IsValid() {
		valelem = reflect.Zero(p.mtype.Elem())
	}

	v.SetMapIndex(keyelem, valelem)
	return nil
}

// Decode a group.
func (o *Buffer) dec_struct_group(p *Properties, base structPointer) error {
	bas := structPointer_GetStructPointer(base, p.field)
	if bas == nil {
		// allocate new nested message
		bas = toStructPointer(reflect.New(p.stype))
		structPointer_SetStructPointer(base, p.field, bas)
	}
	return o.unmarshalType(p.stype, p.sprop, true, bas)
}

// Decode an embedded message.
func (o *Buffer) dec_struct_message(p *Properties, base structPointer) (err error) {
	raw, e := o.DecodeRawBytes(false)
	if e != nil {
		return e
	}

	var bas structPointer
	if p.isPtr {
		bas = structPointer_GetStructPointer(base, p.field)
	} else {
		bas = (structPointer)(unsafe.Pointer(uintptr(base) + uintptr(p.field)))
	}

	if bas == nil {
		// allocate new nested message
		bas = toStructPointer(reflect.New(p.stype))
		structPointer_SetStructPointer(base, p.field, bas)
	}

	obuf := o.buf
	oi := o.index
	o.buf = raw
	o.index = 0

	err = o.unmarshalType(p.stype, p.sprop, false, bas)
	o.buf = obuf
	o.index = oi

	return err
}

func (o *Buffer) dec_slice_struct_message_s(p *Properties, base structPointer) error {
	v := reflect.New(p.stype)
	bas := toStructPointer(v)

	raw, e := o.DecodeRawBytes(false)
	if e != nil {
		return e
	}

	obuf := o.buf
	oi := o.index
	o.buf = raw
	o.index = 0

	err := o.unmarshalType(p.stype, p.sprop, false, bas)
	o.buf = obuf
	o.index = oi

	s := structPointer_NewAt(base, p.field, p.mtype).Elem() // slice
	s1 := reflect.Append(s, v.Elem())
	s.Set(s1)

	return err
}

// Decode a slice of embedded messages.
func (o *Buffer) dec_slice_struct_message(p *Properties, base structPointer) error {
	return o.dec_slice_struct(p, false, base)
}

// Decode a slice of embedded groups.
func (o *Buffer) dec_slice_struct_group(p *Properties, base structPointer) error {
	return o.dec_slice_struct(p, true, base)
}

// Decode a slice of structs ([]*struct).
func (o *Buffer) dec_slice_struct(p *Properties, is_group bool, base structPointer) error {
	v := reflect.New(p.stype)
	bas := toStructPointer(v)
	(*structPointerSlice)(unsafe.Pointer(uintptr(base) + uintptr(p.field))).Append(bas)

	if is_group {
		err := o.unmarshalType(p.stype, p.sprop, is_group, bas)
		return err
	}

	raw, err := o.DecodeRawBytes(false)
	if err != nil {
		return err
	}

	obuf := o.buf
	oi := o.index
	o.buf = raw
	o.index = 0

	err = o.unmarshalType(p.stype, p.sprop, is_group, bas)

	o.buf = obuf
	o.index = oi

	return err
}
