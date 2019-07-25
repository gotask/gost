package stutil

import (
	"io"
	"unsafe"
)

func RUint16(b []byte) (x uint16) {
	_ = b[1]
	x = uint16(b[0])
	x |= uint16(b[1]) << 8
	return
}

func WUint16(b []byte, x uint16) {
	_ = b[1]
	b[0] = byte(x)
	b[1] = byte(x >> 8)
}

func RInt16(b []byte) int16 {
	v := RUint16(b)
	return int16(v)
}

func WInt16(b []byte, x int16) {
	WUint16(b, uint16(x))
}

func RUint32(b []byte) (x uint32) {
	_ = b[3]
	x = uint32(b[0])
	x |= uint32(b[1]) << 8
	x |= uint32(b[2]) << 16
	x |= uint32(b[3]) << 24
	return
}

func WUint32(b []byte, x uint32) {
	_ = b[3]
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
}

func RInt32(b []byte) int32 {
	v := RUint32(b)
	return int32(v)
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

func RUint64(b []byte) (x uint64) {
	_ = b[7]
	x = uint64(b[0])
	x |= uint64(b[1]) << 8
	x |= uint64(b[2]) << 16
	x |= uint64(b[3]) << 24
	x |= uint64(b[4]) << 32
	x |= uint64(b[5]) << 40
	x |= uint64(b[6]) << 48
	x |= uint64(b[7]) << 56
	return
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
	v := RUint64(b)
	return int64(v)
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

func RVarint(b []byte, x uint64) (int, error) {
	var n int
	for n = 0; x > 127; n++ {
		if n >= len(b) {
			return 0, io.ErrShortBuffer
		}
		b[n] = 0x80 | uint8(x&0x7F)
		x >>= 7
	}
	if n >= len(b) {
		return 0, io.ErrShortBuffer
	}
	b[n] = uint8(x)
	n++
	return n, nil
}

func WVarint(b []byte) (x uint64, n int, e error) {
	// x, n already 0
	for shift := uint(0); shift < 64; shift += 7 {
		if n >= len(b) {
			return 0, 0, io.ErrUnexpectedEOF
		}
		b := uint64(b[n])
		n++
		x |= (b & 0x7F) << shift
		if (b & 0x80) == 0 {
			return x, n, nil
		}
	}

	// The number is too large to represent in a 64-bit value.
	return 0, 0, io.ErrUnexpectedEOF
}
