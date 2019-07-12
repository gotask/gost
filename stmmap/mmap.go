// mmap.go
package stmmap

import (
	"os"
)

type Mmap interface {
	Data() []byte
	Size() uint32
	Flush() error
	Lock() error
	Unlock() error
	Unmap() error
}

func NewMmap(f *os.File, offset int64, length int) (Mmap, error) {
	m := &mmap{}
	e := m.Init(int(f.Fd()), offset, length)
	if e != nil {
		return nil, e
	}
	return m, nil
}

type mmap struct {
	data []byte
	size uint32
	h    uintptr
}

func (mp *mmap) Data() []byte {
	return mp.data
}
func (mp *mmap) Size() uint32 {
	return mp.size
}

func (mp *mmap) reset() {
	mp.data = nil
	mp.size = 0
	mp.h = 0
}
