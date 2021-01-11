package stkvbuffer

import (
	"reflect"
	"unsafe"
)

func read_uint32(base []byte) uint32 {
	return *(*uint32)(unsafe.Pointer(&base[0]))
}
func read_uint64(base []byte) uint64 {
	return *(*uint64)(unsafe.Pointer(&base[0]))
}

func write_uint32(base []byte, i uint32) {
	*(*uint32)(unsafe.Pointer(&base[0])) = i
}
func write_uint64(base []byte, i uint64) {
	*(*uint64)(unsafe.Pointer(&base[0])) = i
}

func bytesToStringUnsafe(b []byte) string {
	if b == nil {
		return ""
	}
	return *(*string)(unsafe.Pointer(&b))
}

func stringToBytesUnsafe(s string) []byte {
	if s == "" {
		return nil
	}
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{sh.Data, sh.Len, sh.Len}
	return *(*[]byte)(unsafe.Pointer(&bh))
}
