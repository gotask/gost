package stnet

type BufferPool struct {
}

var bp BufferPool

func (bp *BufferPool) Alloc(bufsize int) []byte {
	return make([]byte, bufsize)
}

func (bp *BufferPool) Free([]byte) {
}
