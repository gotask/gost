package stencode

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unsafe"
)

// Constants that identify the encoding of a value on the wire.
const (
	WireVarint     = 0
	WireFixed64    = 1
	WireBytes      = 2
	WireStartGroup = 3
	WireEndGroup   = 4
	WireFixed32    = 5
)

var (
	propertiesMu  sync.RWMutex
	propertiesMap = make(map[reflect.Type]*StructProperties)
)

type structPointer unsafe.Pointer

func toStructPointer(v reflect.Value) structPointer {
	return structPointer(unsafe.Pointer(v.Pointer()))
}

func structPointer_NewAt(p structPointer, f uintptr, typ reflect.Type) reflect.Value {
	return reflect.NewAt(typ, unsafe.Pointer(uintptr(p)+uintptr(f)))
}

// SetStructPointer writes a *struct field in the struct.
func structPointer_SetStructPointer(p structPointer, f uintptr, q structPointer) {
	*(*structPointer)(unsafe.Pointer(uintptr(p) + uintptr(f))) = q
}

// GetStructPointer reads a *struct field in the struct.
func structPointer_GetStructPointer(p structPointer, f uintptr) structPointer {
	return *(*structPointer)(unsafe.Pointer(uintptr(p) + uintptr(f)))
}

// Encoders are defined in encode.go
// An encoder outputs the full representation of a field, including its
// tag and encoder type.
type encoder func(p *Buffer, prop *Properties, base structPointer) error

// A valueEncoder encodes a single integer in a particular encoding.
type valueEncoder func(o *Buffer, x uint64) error

// Sizers are defined in encode.go
// A sizer returns the encoded size of a field, including its tag and encoder
// type.
type sizer func(prop *Properties, base structPointer) int

// A valueSizer returns the encoded size of a single integer in a particular
// encoding.
type valueSizer func(x uint64) int

// Decoders are defined in decode.go
// A decoder creates a value from its wire representation.
// Unrecognized subelements are saved in unrec.
type decoder func(p *Buffer, prop *Properties, base structPointer) error

// A valueDecoder decodes a single integer in a particular encoding.
type valueDecoder func(o *Buffer) (x uint64, err error)

type Properties struct {
	Name     string // name of the field, for error messages
	OrigName string // original name before protocol compiler (always set)
	JSONName string // name to use for JSON; determined by protoc
	Wire     string
	WireType int
	Tag      int

	proto3 bool // whether this is known to be a proto3 field; set for []byte only

	enc     encoder
	valEnc  valueEncoder // set for bool and numeric types only
	field   uintptr
	tagcode []byte // encoding of EncodeVarint((Tag<<3)|WireType)
	tagbuf  [8]byte
	stype   reflect.Type      // set for struct types only
	sprop   *StructProperties // set for struct types only

	mtype    reflect.Type // set for map types only
	mkeyprop *Properties  // set for map types only
	mvalprop *Properties  // set for map types only

	size    sizer
	valSize valueSizer // set for bool and numeric types only

	dec    decoder
	valDec valueDecoder // set for bool and numeric types only

	isPtr bool
}

type StructProperties struct {
	Prop []*Properties // properties for each field
	//decoderTags      tagMap         // map from proto tag to struct field number
	decoderOrigNames map[string]int // map from original name to struct field number
	order            []int          // list of struct field numbers in tag order
	stype            reflect.Type
}

func (sp *StructProperties) Len() int { return len(sp.order) }
func (sp *StructProperties) Less(i, j int) bool {
	return sp.Prop[sp.order[i]].Tag < sp.Prop[sp.order[j]].Tag
}
func (sp *StructProperties) Swap(i, j int) { sp.order[i], sp.order[j] = sp.order[j], sp.order[i] }

func GetProperties(t reflect.Type) *StructProperties {
	if t.Kind() != reflect.Struct {
		panic("proto: type must have kind struct")
	}

	// Most calls to GetProperties in a long-running program will be
	// retrieving details for types we have seen before.
	propertiesMu.RLock()
	sprop, ok := propertiesMap[t]
	propertiesMu.RUnlock()
	if ok {
		return sprop
	}

	propertiesMu.Lock()
	sprop = getPropertiesLocked(t)
	propertiesMu.Unlock()
	return sprop
}

func getTag(f reflect.StructField, i int) string {
	styp := ""
	switch f.Type.Kind() {
	case reflect.Bool, reflect.Int, reflect.Uint8, reflect.Int32, reflect.Uint32, reflect.Int64, reflect.Uint64:
		styp = "varint"
	case reflect.Float32:
		styp = "fixed32"
	case reflect.String, reflect.Map:
		styp = "bytes"
	case reflect.Slice:
		switch t2 := f.Type.Elem(); t2.Kind() {
		default:
			return ""
		case reflect.Bool, reflect.Int, reflect.Int32, reflect.Uint32, reflect.Int64, reflect.Uint64:
			styp = "varint"
		case reflect.Float32:
			styp = "fixed32"
		case reflect.String, reflect.Uint8:
			styp = "bytes"
		case reflect.Ptr:
			switch t3 := t2.Elem(); t3.Kind() {
			default:
				return ""
			case reflect.Struct:
				styp = "bytes"
			}
		case reflect.Slice:
			switch t2.Elem().Kind() {
			default:
				return ""
			case reflect.Uint8:
				styp = "bytes"
			}
		}
	case reflect.Ptr:
		switch t2 := f.Type.Elem(); t2.Kind() {
		default:
			return ""
		case reflect.Struct:
			styp = "bytes"
		}
	default:
		return ""
	}
	return styp + "," + strconv.Itoa(i)
}

func getPropertiesLocked(t reflect.Type) *StructProperties {
	if prop, ok := propertiesMap[t]; ok {
		return prop
	}

	prop := new(StructProperties)
	// in case of recursive protos, fill this in now.
	propertiesMap[t] = prop

	prop.Prop = make([]*Properties, t.NumField())
	prop.order = make([]int, t.NumField())

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		p := new(Properties)
		name := f.Name
		//p.init(f.Type, name, f.Tag.Get("protobuf"), &f, false)
		p.init(f.Type, name, getTag(f, i+1), &f, false)

		prop.Prop[i] = p
		prop.order[i] = i
	}

	// Re-order prop.order.
	sort.Sort(prop)

	return prop
}

func logNoSliceEnc(t1, t2 reflect.Type) {
	fmt.Fprintf(os.Stderr, "proto: no slice oenc for %T = []%T\n", t1, t2)
}

func (p *Properties) init(typ reflect.Type, name, tag string, f *reflect.StructField, lockGetProp bool) {
	// "bytes,49,opt,def=hello!"
	p.Name = name
	p.OrigName = name
	if f != nil {
		p.field = f.Offset
	}
	if tag == "" {
		return
	}
	p.Parse(tag)
	p.setEncAndDec(typ, f, lockGetProp)
}

func (p *Properties) Parse(s string) {
	// "bytes,49,opt,name=foo,def=hello!"
	fields := strings.Split(s, ",") // breaks def=, but handled below.
	if len(fields) < 2 {
		fmt.Fprintf(os.Stderr, "proto: tag has too few fields: %q\n", s)
		return
	}

	p.Wire = fields[0]
	switch p.Wire {
	case "varint":
		p.WireType = WireVarint
		p.valEnc = (*Buffer).EncodeVarint
		p.valDec = (*Buffer).DecodeVarint
		p.valSize = sizeVarint
	case "fixed32":
		p.WireType = WireFixed32
		p.valEnc = (*Buffer).EncodeFixed32
		p.valDec = (*Buffer).DecodeFixed32
		p.valSize = sizeFixed32
	case "fixed64":
		p.WireType = WireFixed64
		p.valEnc = (*Buffer).EncodeFixed64
		p.valDec = (*Buffer).DecodeFixed64
		p.valSize = sizeFixed64
	case "zigzag32":
		p.WireType = WireVarint
		p.valEnc = (*Buffer).EncodeZigzag32
		p.valDec = (*Buffer).DecodeZigzag32
		p.valSize = sizeZigzag32
	case "zigzag64":
		p.WireType = WireVarint
		p.valEnc = (*Buffer).EncodeZigzag64
		p.valDec = (*Buffer).DecodeZigzag64
		p.valSize = sizeZigzag64
	case "bytes", "group":
		p.WireType = WireBytes
		// no numeric converter for non-numeric types
	default:
		fmt.Fprintf(os.Stderr, "proto: tag has unknown wire type: %q\n", s)
		return
	}

	var err error
	p.Tag, err = strconv.Atoi(fields[1])
	if err != nil {
		return
	}
}

// Initialize the fields for encoding and decoding.
func (p *Properties) setEncAndDec(typ reflect.Type, f *reflect.StructField, lockGetProp bool) {
	p.enc = nil
	p.dec = nil
	p.size = nil

	k := typ.Kind()
	if k == reflect.Ptr {
		p.isPtr = true
		k = typ.Elem().Kind()
	}

	switch k {
	default:
		fmt.Fprintf(os.Stderr, "proto: no coders for %v\n", typ)

	case reflect.Bool:
		p.enc = (*Buffer).enc_bool
		p.dec = (*Buffer).dec_bool
		p.size = size_bool
	case reflect.Int:
		p.enc = (*Buffer).enc_int
		p.dec = (*Buffer).dec_int
		p.size = size_int32
	case reflect.Int32:
		p.enc = (*Buffer).enc_int32
		p.dec = (*Buffer).dec_int32
		p.size = size_int32
	case reflect.Uint32:
		p.enc = (*Buffer).enc_uint32
		p.dec = (*Buffer).dec_int32 // can reuse
		p.size = size_uint32
	case reflect.Int64, reflect.Uint64:
		p.enc = (*Buffer).enc_int64
		p.dec = (*Buffer).dec_int64
		p.size = size_int64
	case reflect.Float32:
		p.enc = (*Buffer).enc_uint32 // can just treat them as bits
		p.dec = (*Buffer).dec_int32
		p.size = size_uint32
	case reflect.Float64:
		p.enc = (*Buffer).enc_int64 // can just treat them as bits
		p.dec = (*Buffer).dec_int64
		p.size = size_int64
	case reflect.String:
		p.enc = (*Buffer).enc_string
		p.dec = (*Buffer).dec_string
		p.size = size_string
	case reflect.Struct:
		p.stype = typ
		if p.isPtr {
			p.stype = typ.Elem()
		}
		p.enc = (*Buffer).enc_struct_message
		p.dec = (*Buffer).dec_struct_message
		p.size = size_struct_message

	case reflect.Slice:
		switch t2 := typ.Elem(); t2.Kind() {
		default:
			logNoSliceEnc(typ, t2)
			break
		case reflect.Bool:
			p.enc = (*Buffer).enc_slice_bool
			p.size = size_slice_bool
			p.dec = (*Buffer).dec_slice_bool
		case reflect.Int32:
			p.enc = (*Buffer).enc_slice_int32
			p.size = size_slice_int32
			p.dec = (*Buffer).dec_slice_int32
		case reflect.Uint32:
			p.enc = (*Buffer).enc_slice_uint32
			p.size = size_slice_uint32
			p.dec = (*Buffer).dec_slice_int32
		case reflect.Int64, reflect.Uint64:
			p.enc = (*Buffer).enc_slice_int64
			p.size = size_slice_int64
			p.dec = (*Buffer).dec_slice_int64
		case reflect.Uint8:
			p.dec = (*Buffer).dec_slice_byte
			p.enc = (*Buffer).enc_slice_byte
			p.size = size_slice_byte
		case reflect.Float32, reflect.Float64:
			switch t2.Bits() {
			case 32:
				// can just treat them as bits
				p.enc = (*Buffer).enc_slice_uint32
				p.size = size_slice_uint32
				p.dec = (*Buffer).dec_slice_int32
			case 64:
				// can just treat them as bits
				p.enc = (*Buffer).enc_slice_int64
				p.size = size_slice_int64
				p.dec = (*Buffer).dec_slice_int64
			default:
				logNoSliceEnc(typ, t2)
				break
			}
		case reflect.String:
			p.enc = (*Buffer).enc_slice_string
			p.dec = (*Buffer).dec_slice_string
			p.size = size_slice_string
		case reflect.Ptr:
			switch t3 := t2.Elem(); t3.Kind() {
			default:
				fmt.Fprintf(os.Stderr, "proto: no ptr oenc for %T -> %T -> %T\n", typ, t2, t3)
				break
			case reflect.Struct:
				p.stype = t2.Elem()
				p.enc = (*Buffer).enc_slice_struct_message
				p.dec = (*Buffer).dec_slice_struct_message
				p.size = size_slice_struct_message
			}
		case reflect.Slice:
			switch t2.Elem().Kind() {
			default:
				fmt.Fprintf(os.Stderr, "proto: no slice elem oenc for %T -> %T -> %T\n", typ, t2, t2.Elem())
				break
			case reflect.Uint8:
				p.enc = (*Buffer).enc_slice_slice_byte
				p.dec = (*Buffer).dec_slice_slice_byte
				p.size = size_slice_slice_byte
			}
		}

	case reflect.Map:
		p.enc = (*Buffer).enc_new_map
		p.dec = (*Buffer).dec_new_map
		p.size = size_new_map

		p.mtype = typ
		p.mkeyprop = &Properties{}
		p.mkeyprop.init(reflect.PtrTo(p.mtype.Key()), "Key", f.Tag.Get("protobuf_key"), nil, lockGetProp)
		p.mvalprop = &Properties{}
		vtype := p.mtype.Elem()
		if vtype.Kind() != reflect.Ptr && vtype.Kind() != reflect.Slice {
			// The value type is not a message (*T) or bytes ([]byte),
			// so we need encoders for the pointer to this type.
			vtype = reflect.PtrTo(vtype)
		}
		p.mvalprop.init(vtype, "Value", f.Tag.Get("protobuf_val"), nil, lockGetProp)
	}

	// precalculate tag code
	wire := p.WireType
	x := uint32(p.Tag)<<3 | uint32(wire)
	i := 0
	for i = 0; x > 127; i++ {
		p.tagbuf[i] = 0x80 | uint8(x&0x7F)
		x >>= 7
	}
	p.tagbuf[i] = uint8(x)
	p.tagcode = p.tagbuf[0 : i+1]

	if p.stype != nil {
		if lockGetProp {
			p.sprop = GetProperties(p.stype)
		} else {
			p.sprop = getPropertiesLocked(p.stype)
		}
	}
}
