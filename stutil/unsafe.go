package stutil

import (
	"unsafe"
)

func RUint16(b []byte) uint16 {
	_ = b[1]
	var v uint16
	v = *(*uint16)(unsafe.Pointer(&b[0]))
	return v
}

func WUint16(b []byte, x uint16) {
	_ = b[1]
	b[0] = byte(x)
	b[1] = byte(x >> 8)
}

func RInt16(b []byte) int16 {
	_ = b[1]
	var v int16
	v = *(*int16)(unsafe.Pointer(&b[0]))
	return v
}

func WInt16(b []byte, x int16) {
	WUint16(b, uint16(x))
}

func RUint32(b []byte) uint32 {
	_ = b[3]
	var v uint32
	v = *(*uint32)(unsafe.Pointer(&b[0]))
	return v
}

func WUint32(b []byte, x uint32) {
	_ = b[3]
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
}

func RInt32(b []byte) int32 {
	_ = b[3]
	var v int32
	v = *(*int32)(unsafe.Pointer(&b[0]))
	return v
}

func WInt32(b []byte, x int32) {
	WUint32(b, uint32(x))
}

func RFloat32(b []byte) float32 {
	v := RUint32(b)
	return *(*float32)(unsafe.Pointer(&v))
}

func WFloat32(b []byte, x float32) {
	v := *(*uint32)(unsafe.Pointer(&x))
	WUint32(b, v)
}

func RUint64(b []byte) uint64 {
	_ = b[7]
	var v uint64
	v = *(*uint64)(unsafe.Pointer(&b[0]))
	return v
}

func WUint64(b []byte, x uint64) {
	_ = b[3]
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
	b[4] = byte(x >> 32)
	b[5] = byte(x >> 40)
	b[6] = byte(x >> 48)
	b[7] = byte(x >> 56)
}

func RInt64(b []byte) int64 {
	_ = b[3]
	var v int64
	v = *(*int64)(unsafe.Pointer(&b[0]))
	return v
}

func WInt64(b []byte, x int64) {
	WUint64(b, uint64(x))
}

func RFloat64(b []byte) float64 {
	v := RUint64(b)
	return *(*float64)(unsafe.Pointer(&v))
}

func WFloat64(b []byte, x float64) {
	v := *(*uint64)(unsafe.Pointer(&x))
	WUint64(b, v)
}
