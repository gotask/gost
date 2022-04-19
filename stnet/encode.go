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

func MsgLen(b []byte) uint32 {
	return uint32(b[3]) | uint32(b[2])<<8 | uint32(b[1])<<16 | uint32(b[0])<<24
}

func EncodeProtocol(msg interface{}, encode int) ([]byte, error) {
	data, e := Marshal(msg, encode)
	if e != nil {
		return nil, e
	}
	msglen := len(data) + 4
	buff := make([]byte, msglen)
	buff[0] = byte(msglen >> 24)
	buff[1] = byte(msglen >> 18)
	buff[2] = byte(msglen >> 8)
	buff[3] = byte(msglen)
	copy(buff[4:], data)
	return buff, nil
}
