package stnet

import (
	"errors"
	"io"
	"reflect"
	"strconv"
	"unsafe"
)

var (
	errNoEnoughData       = errors.New("NoEnoughData")
	errOverflow           = errors.New("integer overflow")
	errInvalidType        = errors.New("invalid type")
	errStructEnd          = errors.New("struct end")
	errNeedPtr            = errors.New("ptr is needed")
	errFiledTypeUnmatched = errors.New("field type unmatched")
)

//`tag:"0" require:"true"`

type Spb struct {
	buf   []byte // encode/decode byte stream
	index int    // write/read point
}

const (
	SpbPackDataType_Integer_Positive = 0
	SpbPackDataType_Integer_Negative = 1
	SpbPackDataType_Float            = 2
	SpbPackDataType_Double           = 3
	SpbPackDataType_String           = 4
	SpbPackDataType_Vector           = 5
	SpbPackDataType_Map              = 6
	SpbPackDataType_StructBegin      = 7
	SpbPackDataType_StructEnd        = 8
)

func (spb *Spb) Buffer() []byte {
	return spb.buf
}

func (spb *Spb) packData(data []byte) {
	spb.buf = append(spb.buf, data...)
}

func (spb *Spb) packByte(x byte) {
	spb.buf = append(spb.buf, x)
}

func (spb *Spb) packNumber(x uint64) {
	for x >= 1<<7 {
		spb.buf = append(spb.buf, uint8(x&0x7f|0x80))
		x >>= 7
	}
	spb.buf = append(spb.buf, uint8(x))
}

func (spb *Spb) packHeader(tag uint32, typ uint8) {
	header := typ << 4
	if tag < 15 {
		header = header | uint8(tag)
		spb.packByte(header)
	} else {
		header = header | 0xf
		spb.packByte(header)
		spb.packNumber(uint64(tag))
	}
}

func (spb *Spb) packSlice(tag uint32, x interface{}, packHead bool, require bool) error {
	if reflect.TypeOf(x).Kind() != reflect.Slice {
		return errInvalidType
	}
	refVal := reflect.ValueOf(x)
	if refVal.Len() == 0 && !require {
		return nil
	}
	spb.packHeader(tag, SpbPackDataType_Vector)
	spb.packNumber(uint64(refVal.Len()))
	for i := 0; i < refVal.Len(); i++ {
		err := spb.pack(0, refVal.Index(i).Interface(), true, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func (spb *Spb) packMap(tag uint32, x interface{}, packHead bool, require bool) error {
	if reflect.TypeOf(x).Kind() != reflect.Map {
		return errInvalidType
	}
	refVal := reflect.ValueOf(x)
	if refVal.Len() == 0 && !require {
		return nil
	}
	spb.packHeader(tag, SpbPackDataType_Map)
	spb.packNumber(uint64(refVal.Len()))
	keys := refVal.MapKeys()
	for i := 0; i < len(keys); i++ {
		err := spb.pack(0, keys[i].Interface(), true, true)
		if err != nil {
			return err
		}
		err = spb.pack(0, refVal.MapIndex(keys[i]).Interface(), true, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func (spb *Spb) packStruct(tag uint32, x interface{}, packHead bool) error {
	refVal := reflect.ValueOf(x)
	if reflect.TypeOf(x).Kind() == reflect.Ptr {
		refVal = refVal.Elem()
	}
	if refVal.Kind() != reflect.Struct {
		return errInvalidType
	}
	spb.packHeader(tag, SpbPackDataType_StructBegin)

	if !refVal.IsZero() {
		for i := 0; i < refVal.NumField(); i++ {
			fld := refVal.Field(i)
			if !fld.CanInterface() {
				continue
			}

			iTg := i
			tg := refVal.Type().Field(i).Tag.Get("tag")
			if tg != "" {
				itg, er := strconv.Atoi(tg)
				if er == nil {
					iTg = itg
				}
			}
			req := refVal.Type().Field(i).Tag.Get("require")
			require := false
			if req == "true" {
				require = true
			}

			err := spb.pack(uint32(iTg), fld.Interface(), true, require)
			if err != nil {
				return err
			}
		}
	}

	spb.packHeader(0, SpbPackDataType_StructEnd)
	return nil
}

func (spb *Spb) pack(tag uint32, i interface{}, packHead bool, require bool) error {
	t := reflect.TypeOf(i)
	v := reflect.ValueOf(i)
	var x interface{}
	if t.Kind() == reflect.Ptr {
		if v.IsNil() {
			v = newValByType(t)
		}
		x = v.Elem().Interface()
	} else {
		x = v.Interface()
	}
	if x == nil {
		return errors.New("Marshal called with nil")
	}

	typ := SpbPackDataType_Integer_Positive
	var val uint64

	switch reflect.TypeOf(x).Kind() {
	case reflect.Bool:
		{
			v := x.(bool)
			if v {
				val = 1
			}
		}
	case reflect.Int:
		{
			v := x.(int)
			if v < 0 {
				typ = SpbPackDataType_Integer_Negative
				v = -v
			}
			val = uint64(v)
		}
	case reflect.Int8:
		{
			v := x.(int8)
			if v < 0 {
				typ = SpbPackDataType_Integer_Negative
				v = -v
			}
			val = uint64(v)
		}
	case reflect.Int16:
		{
			v := x.(int16)
			if v < 0 {
				typ = SpbPackDataType_Integer_Negative
				v = -v
			}
			val = uint64(v)
		}
	case reflect.Int32:
		{
			v := x.(int32)
			if v < 0 {
				typ = SpbPackDataType_Integer_Negative
				v = -v
			}
			val = uint64(v)
		}
	case reflect.Int64:
		{
			v := x.(int64)
			if v < 0 {
				typ = SpbPackDataType_Integer_Negative
				v = -v
			}
			val = uint64(v)
		}
	case reflect.Uint:
		{
			v := x.(uint)
			val = uint64(v)
		}
	case reflect.Uint8:
		{
			v := x.(uint8)
			val = uint64(v)
		}
	case reflect.Uint16:
		{
			v := x.(uint16)
			val = uint64(v)
		}
	case reflect.Uint32:
		{
			v := x.(uint32)
			val = uint64(v)
		}
	case reflect.Uint64:
		{
			v := x.(uint64)
			val = v
		}
	case reflect.Float32:
		{
			typ = SpbPackDataType_Float
			v := x.(float32)
			val = uint64(*(*uint32)(unsafe.Pointer(&v)))
		}
	case reflect.Float64:
		{
			typ = SpbPackDataType_Double
			v := x.(float64)
			val = *(*uint64)(unsafe.Pointer(&v))
		}
	case reflect.String:
		{
			v := x.(string)
			if len(v) == 0 && !require {
				return nil
			}
			if packHead {
				spb.packHeader(tag, SpbPackDataType_String)
			}
			spb.packNumber(uint64(len(v)))
			spb.packData(*(*[]byte)(unsafe.Pointer(&v)))
			return nil
		}
	case reflect.Slice:
		{
			return spb.packSlice(tag, x, packHead, require)
		}
	case reflect.Map:
		{
			return spb.packMap(tag, x, packHead, require)
		}
	case reflect.Struct:
		{
			return spb.packStruct(tag, x, packHead)
		}
	default:
		{
			return errInvalidType
		}
	}

	if val == 0 && !require {
		return nil
	}

	if packHead {
		spb.packHeader(tag, uint8(typ))
	}
	spb.packNumber(val)
	return nil
}

func (spb *Spb) unpackByte(n int) (x []byte, err error) {
	if n <= 0 {
		return nil, nil
	}
	if spb.index+n > len(spb.buf) {
		return nil, errNoEnoughData
	}
	x = make([]byte, n)
	copy(x, spb.buf[spb.index:spb.index+n])
	spb.index = spb.index + n
	return
}

func (spb *Spb) unpackNumber() (x uint64, err error) {
	// x, err already 0

	i := spb.index
	l := len(spb.buf)

	for shift := uint(0); shift < 64; shift += 7 {
		if i >= l {
			err = io.ErrUnexpectedEOF
			return
		}
		b := spb.buf[i]
		i++
		x |= (uint64(b) & 0x7F) << shift
		if b < 0x80 {
			spb.index = i
			return
		}
	}

	// The number is too large to represent in a 64-bit value.
	err = errOverflow
	return
}

func (spb *Spb) unpackHeader() (tag uint32, typ uint8, err error) {
	if len(spb.buf) <= spb.index {
		return 0, 0, errNoEnoughData
	}
	typ = spb.buf[spb.index] >> 4
	tag = uint32(spb.buf[spb.index] & 0xf)
	spb.index++
	if tag == 0xf {
		tag1, err1 := spb.unpackNumber()
		return uint32(tag1), typ, err1
	}
	return
}

func CanSetBool(x reflect.Value) bool {
	if !x.CanSet() {
		return false
	}
	switch k := x.Kind(); k {
	default:
		return false
	case reflect.Bool:
		return true
	}
}

func CanSetFloat(x reflect.Value) bool {
	if !x.CanSet() {
		return false
	}
	switch k := x.Kind(); k {
	default:
		return false
	case reflect.Float32, reflect.Float64:
		return true
	}
}

func CanSetInt(x reflect.Value) bool {
	if !x.CanSet() {
		return false
	}
	switch k := x.Kind(); k {
	default:
		return false
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	}
}

func CanSetUint(x reflect.Value) bool {
	if !x.CanSet() {
		return false
	}
	switch k := x.Kind(); k {
	default:
		return false
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	}
}

func (spb *Spb) skipHeadField() {
	_, typ, err := spb.unpackHeader()
	if err != nil {
		return
	}
	spb.skipField(typ)
}

func (spb *Spb) skipField(typ uint8) {
	switch typ {
	case SpbPackDataType_Integer_Positive, SpbPackDataType_Integer_Negative, SpbPackDataType_Float, SpbPackDataType_Double:
		{
			spb.unpackNumber()
		}
	case SpbPackDataType_String:
		{
			ln, err := spb.unpackNumber()
			if err != nil {
				return
			}
			spb.unpackByte(int(ln))
		}
	case SpbPackDataType_Vector:
		{
			ln, err := spb.unpackNumber()
			if err != nil {
				return
			}
			for i := 0; i < int(ln); i++ {
				spb.skipHeadField()
			}
		}
	case SpbPackDataType_Map:
		{
			ln, err := spb.unpackNumber()
			if err != nil {
				return
			}
			for i := 0; i < int(ln); i++ {
				spb.skipHeadField()
				spb.skipHeadField()
			}
		}
	case SpbPackDataType_StructBegin:
		{
			spb.skipToStructEnd()
		}
	case SpbPackDataType_StructEnd:
		{
			break
		}
	default:
		return
	}
}

func (spb *Spb) skipToStructEnd() {
	for {
		_, typ, err := spb.unpackHeader()
		if err != nil {
			return
		}
		if typ == SpbPackDataType_StructEnd {
			break
		}
		spb.skipField(typ)
	}
}

func (spb *Spb) unpack(x reflect.Value, first bool) error {
	if spb.index >= len(spb.buf) {
		return errNoEnoughData
	}

	tag, typ, err := spb.unpackHeader()
	if err != nil {
		return err
	}

	if x.Kind() == reflect.Ptr {
		x = x.Elem()
	}

	var valField reflect.Value
	if !first && typ != SpbPackDataType_StructEnd && x.Type().Kind() == reflect.Struct {
		isTag := false
		for i := 0; i < x.NumField(); i++ {
			tg := x.Type().Field(i).Tag.Get("tag")
			if tg != "" {
				iTg, er := strconv.Atoi(tg)
				if er == nil && iTg == int(tag) {
					valField = x.Field(i)
					isTag = true
					break
				}
			}
		}

		if !isTag {
			t := int(tag)
			if t < x.NumField() {
				tg := x.Type().Field(t).Tag.Get("tag")
				if tg == "" {
					valField = x.Field(t)
					isTag = true
				}
			}
		}

		if !isTag { //not found field
			spb.skipField(typ)
			return nil
		}
	}

	switch typ {
	case SpbPackDataType_Integer_Positive:
		{
			v, err := spb.unpackNumber()
			if err != nil {
				return err
			}

			if first {
				if CanSetUint(x) {
					x.SetUint(v)
				} else if CanSetInt(x) {
					x.SetInt(int64(v))
				} else if CanSetBool(x) {
					x.SetBool(v > 0)
				} else {
					return errFiledTypeUnmatched
				}
			} else {
				if CanSetUint(valField) {
					valField.SetUint(v)
				} else if CanSetInt(valField) {
					valField.SetInt(int64(v))
				} else if CanSetBool(valField) {
					valField.SetBool(v > 0)
				} else {
					return errFiledTypeUnmatched
				}
			}
		}
	case SpbPackDataType_Integer_Negative:
		{
			v, err := spb.unpackNumber()
			if err != nil {
				return err
			}
			vv := int64(-v)

			if first {
				if CanSetInt(x) {
					x.SetInt(vv)
				} else {
					return errFiledTypeUnmatched
				}
			} else {
				if CanSetInt(valField) {
					valField.SetInt(vv)
				} else {
					return errFiledTypeUnmatched
				}
			}
		}
	case SpbPackDataType_Float, SpbPackDataType_Double:
		{
			v, err := spb.unpackNumber()
			if err != nil {
				return err
			}
			f := *(*float64)(unsafe.Pointer(&v))
			if first {
				if CanSetFloat(x) {
					x.SetFloat(f)
				} else {
					return errFiledTypeUnmatched
				}
			} else {
				if CanSetFloat(valField) {
					valField.SetFloat(f)
				} else {
					return errFiledTypeUnmatched
				}
			}
		}
	case SpbPackDataType_String:
		{
			ln, err := spb.unpackNumber()
			if err != nil {
				return err
			}
			bt, er := spb.unpackByte(int(ln))
			if er != nil {
				return er
			}
			str := string(bt)
			if first {
				if x.Kind() == reflect.String {
					x.SetString(str)
				} else {
					return errFiledTypeUnmatched
				}
			} else {
				if valField.Kind() == reflect.String {
					valField.SetString(str)
				} else {
					return errFiledTypeUnmatched
				}
			}
		}
	case SpbPackDataType_Vector:
		{
			ln, err := spb.unpackNumber()
			if err != nil {
				return err
			}
			var vecType reflect.Type
			if first {
				if x.Kind() == reflect.Slice && x.CanSet() {
					x.SetLen(0)
					vecType = x.Type().Elem()
				} else {
					return errFiledTypeUnmatched
				}
			} else {
				if valField.Kind() == reflect.Slice && valField.CanSet() {
					valField.SetLen(0)
					vecType = valField.Type().Elem()
				} else {
					return errFiledTypeUnmatched
				}
			}

			if vecType == nil {
				for i := 0; i < int(ln); i++ {
					spb.skipHeadField()
				}
				break
			}

			vals := make([]reflect.Value, 0, ln)
			for i := 0; i < int(ln); i++ {
				vecVal := newValByType(vecType)
				err := spb.unpack(vecVal, true)
				if err != nil {
					return err
				}
				if vecType.Kind() != reflect.Ptr {
					vecVal = vecVal.Elem()
				}
				vals = append(vals, vecVal)
			}
			vecln := len(vals)
			if first {
				vec := reflect.MakeSlice(x.Type(), vecln, vecln)
				for i, k := range vals {
					vec.Index(i).Set(k)
				}
				x.Set(vec)
			} else {
				vec := reflect.MakeSlice(valField.Type(), vecln, vecln)
				for i, k := range vals {
					vec.Index(i).Set(k)
				}
				valField.Set(vec)
			}
		}
	case SpbPackDataType_Map:
		{
			ln, err := spb.unpackNumber()
			if err != nil {
				return err
			}
			var keyType reflect.Type
			var valType reflect.Type
			if first {
				if x.Kind() == reflect.Map && x.CanSet() {
					keyType = x.Type().Key()
					valType = x.Type().Elem()
				} else {
					return errFiledTypeUnmatched
				}
			} else {
				if valField.Kind() == reflect.Map && valField.CanSet() {
					keyType = valField.Type().Key()
					valType = valField.Type().Elem()
				} else {
					return errFiledTypeUnmatched
				}
			}

			if keyType == nil {
				for i := 0; i < int(ln); i++ {
					spb.skipHeadField()
					spb.skipHeadField()
				}
				break
			}

			valsKey := make([]reflect.Value, 0, ln)
			valsVal := make([]reflect.Value, 0, ln)
			for i := 0; i < int(ln); i++ {
				mapKey := reflect.New(keyType).Elem()
				mapVal := newValByType(valType)
				err := spb.unpack(mapKey, true)
				if err != nil {
					return err
				}
				valsKey = append(valsKey, mapKey)
				er := spb.unpack(mapVal, true)
				if er != nil {
					return er
				}
				if valType.Kind() != reflect.Ptr {
					mapVal = mapVal.Elem()
				}
				valsVal = append(valsVal, mapVal)
			}

			if first {
				mp := reflect.MakeMap(x.Type())
				for i, k := range valsKey {
					mp.SetMapIndex(k, valsVal[i])
				}
				x.Set(mp)
			} else {
				mp := reflect.MakeMap(valField.Type())
				for i, k := range valsKey {
					mp.SetMapIndex(k, valsVal[i])
				}
				valField.Set(mp)
			}
		}
	case SpbPackDataType_StructBegin:
		{
			stVal := x
			if !first {
				if valField.Kind() == reflect.Ptr {
					valField.Set(newValByType(valField.Type().Elem()))
					valField = valField.Elem()
				}
				stVal = valField
			}
			if stVal.Kind() != reflect.Struct {
				return errFiledTypeUnmatched
			}
			for {
				err := spb.unpack(stVal, false)
				if errors.Is(err, errStructEnd) {
					break
				}
				if err != nil {
					return err
				}
			}
		}
	case SpbPackDataType_StructEnd:
		{
			return errStructEnd
		}
	default:
		return errInvalidType
	}

	return nil
}

func newValByType(ty reflect.Type) reflect.Value {
	if ty.Kind() == reflect.Map {
		return reflect.New(reflect.MakeMap(ty).Type())
	} else if ty.Kind() == reflect.Slice {
		return reflect.New(reflect.MakeSlice(ty, 0, 0).Type())
	} else if ty.Kind() == reflect.Ptr {
		return reflect.New(ty.Elem())
	}
	return reflect.New(ty)
}

func SpbEncode(data interface{}) ([]byte, error) {
	spb := Spb{}
	e := spb.pack(0, data, true, true)
	return spb.buf, e
}

func SpbDecode(data []byte, x interface{}) error {
	if reflect.TypeOf(x).Kind() != reflect.Ptr {
		return errNeedPtr
	}
	spb := Spb{data, 0}
	return spb.unpack(reflect.ValueOf(x).Elem(), true)
}
