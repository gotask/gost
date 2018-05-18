package stmysql

import (
	"encoding/json"
)

var (
	marshal   func(v interface{}) ([]byte, error)
	unmarshal func(data []byte, v interface{}) error
)

func init() {
	marshal = json.Marshal
	unmarshal = json.Unmarshal
}

func SetEnc(m func(v interface{}) ([]byte, error), unm func(data []byte, v interface{}) error) {
	marshal = m
	unmarshal = unm
}
