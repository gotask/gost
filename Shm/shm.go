// shm.go
package Shm

type Shm interface {
	Data() []byte
	Size() uint32
	Key() uint32
	Detach() error
	Delete() error
}

func ShmGet(key, size uint32) (Shm, error) {
	s := &shm{}
	e := s.Init(key, size)
	if e != nil {
		return nil, e
	}
	return s, nil
}

type shm struct {
	data []byte
	size uint32
	key  uint32
	name string
	h    uintptr
}

func (sh *shm) Data() []byte {
	return sh.data
}
func (sh *shm) Size() uint32 {
	return sh.size
}
func (sh *shm) Key() uint32 {
	return sh.key
}

func (sh *shm) reset() {
	sh.data = nil
	sh.size = 0
	sh.key = 0
}
