package stutil

import (
	"fmt"
	"io"
)

func EncodeFixed32(b []byte, x uint32) {
	_ = b[3]
	b[0] = byte(x)
	b[1] = byte(x >> 8)
	b[2] = byte(x >> 16)
	b[3] = byte(x >> 24)
}

func DecodeFixed32(b []byte) (x uint32) {
	_ = b[3]
	x = uint32(b[0])
	x |= uint32(b[1]) << 8
	x |= uint32(b[2]) << 16
	x |= uint32(b[3]) << 24

	return
}

func EncodeVarint(b []byte, x uint64) int {
	var n int
	for n = 0; x > 127; n++ {
		if n >= len(b) {
			panic(io.ErrShortBuffer)
		}
		b[n] = 0x80 | uint8(x&0x7F)
		x >>= 7
	}
	if n >= len(b) {
		panic(io.ErrShortBuffer)
	}
	b[n] = uint8(x)
	n++
	return n
}

func DecodeVarint(b []byte) (x uint64, n int) {
	// x, n already 0
	for shift := uint(0); shift < 64; shift += 7 {
		if n >= len(b) {
			panic(io.ErrUnexpectedEOF)
		}
		b := uint64(b[n])
		n++
		x |= (b & 0x7F) << shift
		if (b & 0x80) == 0 {
			return x, n
		}
	}

	// The number is too large to represent in a 64-bit value.
	panic(io.ErrUnexpectedEOF)
	return
}

func EncodeRawBytes(b []byte, s []byte) int {
	l := len(s)
	n := EncodeVarint(b, uint64(l))
	if n+l >= len(b) {
		panic(io.ErrShortBuffer)
	}
	copy(b[n:], s)
	return n + l
}

func DecodeRawBytes(b []byte, alloc bool) (s []byte, alloc_len int) {
	x, n := DecodeVarint(b)

	nb := int(x)
	if nb < 0 {
		panic(fmt.Errorf("bad byte length %d", nb))
	}
	alloc_len = nb + n
	if alloc_len > len(b) {
		panic(io.ErrUnexpectedEOF)
	}

	if !alloc {
		// todo: check if can get more uses of alloc=false
		s = b[n:alloc_len]
		return
	}

	s = make([]byte, int(x))
	copy(s, b[n:alloc_len])
	return
}
