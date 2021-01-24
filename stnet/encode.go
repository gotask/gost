package stnet

import (
	"encoding/json"
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
		v := reflect.ValueOf(m)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if v.Kind() == reflect.Struct {
			return stencode.Marshal(m)
		} else {
			return SpbEncode(m)
		}
	} else if encodeType == 2 {
		return SpbEncode(m)
	} else if encodeType == 3 {
		return json.Marshal(m)
	}
	return SpbEncode(m)
}

func Unmarshal(data []byte, m interface{}, encodeType int) error {
	if encodeType == 1 {
		v := reflect.ValueOf(m)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		if v.Kind() == reflect.Struct {
			return stencode.Unmarshal(data, m)
		} else {
			return SpbDecode(data, m)
		}
	} else if encodeType == 2 {
		return SpbDecode(data, m)
	} else if encodeType == 3 {
		return json.Unmarshal(data, m)
	}
	return SpbDecode(data, m)
}
