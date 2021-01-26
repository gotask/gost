package stnet

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/gotask/gost/stencode"
)

const (
	EncodeTyepGpb  = 1
	EncodeTyepSpb  = 2
	EncodeTyepJson = 3
)

func Marshal(m interface{}, encodeType int) ([]byte, error) {
	if encodeType == 1 {
		if reflect.TypeOf(m).Kind() == reflect.Struct {
			v := reflect.New(reflect.TypeOf(m))
			v.Elem().Set(reflect.ValueOf(m))
			m = v.Interface()
		}
		return stencode.Marshal(m)
	} else if encodeType == 2 {
		return SpbEncode(m)
	} else if encodeType == 3 {
		return json.Marshal(m)
	}
	return nil, fmt.Errorf("error encode type %d", encodeType)
}

func Unmarshal(data []byte, m interface{}, encodeType int) error {
	rv := reflect.ValueOf(m)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("Unmarshal need is ptr,but this is %s", rv.Kind())
	}

	if encodeType == 1 {
		return stencode.Unmarshal(data, m)
	} else if encodeType == 2 {
		return SpbDecode(data, m)
	} else if encodeType == 3 {
		return json.Unmarshal(data, m)
	}
	return fmt.Errorf("error encode type %d", encodeType)
}

func rpcMarshal(spb *Spb, tag uint32, i interface{}) error {
	ptr := false
	t := reflect.TypeOf(i)
	if t.Kind() == reflect.Ptr {
		ptr = true
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return spb.pack(tag, i, true, true)
	} else {
		if !ptr {
			v := reflect.New(reflect.TypeOf(i))
			v.Elem().Set(reflect.ValueOf(i))
			i = v.Interface()
		}

		data, e := stencode.Marshal(i)
		if e != nil {
			return e
		}
		spb.packHeader(tag, SpbPackDataType_StructBegin)
		spb.packNumber(uint64(len(data)))
		spb.packData(data)
	}
	return nil
}

func rpcUnmarshal(spb *Spb, tag uint32, i interface{}) error {
	rv := reflect.ValueOf(i)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("Unmarshal need is ptr,but this is %s", rv.Kind())
	}

	t := reflect.TypeOf(i)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return spb.unpack(rv, true)
	} else {
		tg, tp, err := spb.unpackHeader()
		if err != nil {
			return err
		}
		if tag != tg {
			return fmt.Errorf("Unmarshal tag not equal,%d!=%d", tag, tg)
		}
		if tp != SpbPackDataType_StructBegin {
			return fmt.Errorf("Unmarshal type not struct,is %d", tp)
		}
		l, e := spb.unpackNumber()
		if e != nil {
			return e
		}
		bt, er := spb.unpackByte(int(l))
		if er != nil {
			return er
		}
		e = stencode.Unmarshal(bt, i)
		if e != nil {
			return e
		}
	}
	return nil
}
