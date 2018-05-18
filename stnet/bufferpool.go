package stnet

type BufferPool struct {
}

const maxNBuf = 4096

var bp BufferPool

func (bp *BufferPool) Alloc(bufsize int) []byte {
	return make([]byte, bufsize)
}

func (bp *BufferPool) Free([]byte) {
}
