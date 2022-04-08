package stnet

import (
	"encoding/json"
	"fmt"
	"reflect"
)

const (
	EncodeTyepSpb  = 0
	EncodeTyepJson = 1
)

func Marshal(m interface{}, encodeType int) ([]byte, error) {
	if encodeType == EncodeTyepSpb {
		return SpbEncode(m)
	} else if encodeType == EncodeTyepJson {
		return json.Marshal(m)
	}
	return nil, fmt.Errorf("error encode type %d", encodeType)
}

func Unmarshal(data []byte, m interface{}, encodeType int) error {
	rv := reflect.ValueOf(m)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("Unmarshal need is ptr,but this is %s", rv.Kind())
	}

	if encodeType == EncodeTyepSpb {
		return SpbDecode(data, m)
	} else if encodeType == EncodeTyepJson {
		return json.Unmarshal(data, m)
	}
	return fmt.Errorf("error encode type %d", encodeType)
}

func rpcMarshal(spb *Spb, tag uint32, i interface{}) error {
	return spb.pack(tag, i, true, true)
}

func rpcUnmarshal(spb *Spb, tag uint32, i interface{}) error {
	rv := reflect.ValueOf(i)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return fmt.Errorf("Unmarshal need is ptr,but this is %s", rv.Kind())
	}
	return spb.unpack(rv, true)
}
