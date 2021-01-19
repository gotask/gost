package stkvbuffer

import (
	"container/list"
	"fmt"
)

const (
	HeaderSize int = 16 // 0xFEDCBA98 chunkSize4 dataSize8
)

type Chunk struct {
	data  []byte
	index uint32 //begin with 1
	next  uint32 //0 free =index end
}

type Node struct {
	chunks  []Chunk
	keySize uint32
	valSize uint32
}

func (n *Node) GetKey() []byte {
	key := make([]byte, 0, int(n.keySize))
	chunkSize := uint32(len(n.chunks[0].data))
	if n.keySize <= chunkSize-12 {
		key = append(key, n.chunks[0].data[12:12+n.keySize]...)
	} else {
		key = append(key, n.chunks[0].data[12:]...)
		rest := n.keySize + 12 - chunkSize
		idx := 1
		for rest >= chunkSize-4 {
			key = append(key, n.chunks[idx].data[4:]...)
			idx++
			rest -= chunkSize - 4
		}
		if rest > 0 {
			key = append(key, n.chunks[idx].data[4:4+rest]...)
		}
	}

	return key
}

func (n *Node) getValPos() (idx int, start uint32) {
	chunkSize := uint32(len(n.chunks[0].data))
	if n.keySize < chunkSize-12 {
		start = 12 + n.keySize
	} else {
		rest := n.keySize + 12 - chunkSize
		idx = 1
		for rest >= chunkSize-4 {
			idx++
			rest -= chunkSize - 4
		}
		start = rest + 4
	}
	return
}

func (n *Node) GetVal() []byte {
	val := make([]byte, 0, int(n.valSize))
	chunkSize := uint32(len(n.chunks[0].data))
	idx, start := n.getValPos()

	if n.valSize <= chunkSize-start {
		val = append(val, n.chunks[idx].data[start:start+n.valSize]...)
	} else {
		val = append(val, n.chunks[idx].data[start:]...)
		rest := n.valSize + start - chunkSize
		idx++
		for rest >= chunkSize-4 {
			val = append(val, n.chunks[idx].data[4:]...)
			idx++
			rest -= chunkSize - 4
		}
		if rest > 0 {
			val = append(val, n.chunks[idx].data[4:4+rest]...)
		}
	}
	return val
}

type FuncLruDel func(key string, node Node)

//Buffer implements a kv buffer.
type KVBuffer struct {
	data      []byte
	size      uint64
	chunkSize uint32
	chunkNum  uint32
	freeChunk *list.List
	lru       *LRU
	openLru   bool
	onLruDel  FuncLruDel
}

// NewBuffer constructs a Buffer.
func NewLRUKVBuffer(data []byte, chunkSize int, onLruDel FuncLruDel) (*KVBuffer, error) {
	buf, e := NewKVBuffer(data, chunkSize)
	if e != nil {
		return nil, e
	}
	buf.openLru = true
	buf.onLruDel = onLruDel
	return buf, nil
}

func NewKVBuffer(data []byte, chunkSize int) (*KVBuffer, error) {
	if len(data) < 1024 {
		return nil, fmt.Errorf("data min size is 1024, now is %d", len(data))
	}
	buff := &KVBuffer{data: data, freeChunk: list.New()}
	if data[0] == 0xFE && data[1] == 0xDC && data[2] == 0xBA && data[3] == 0x98 { //is inited
		buff.chunkSize = read_uint32(data[4:])
		if buff.chunkSize != uint32(chunkSize) {
			return nil, fmt.Errorf("chunkSize is wrong,old=%d;new=%d", buff.chunkSize, chunkSize)
		}
		buff.size = read_uint64(data[8:])
		if buff.size > uint64(len(data)) {
			return nil, fmt.Errorf("buff size is wrong,old=%d;new=%d", buff.size, len(data))
		}

		chunkNum := 0
		vChunkEnd := make([]uint32, 0)
		mpChunck := make(map[uint32]Chunk)
		mpChunkPrev := make(map[uint32]uint32)
		for i := HeaderSize; i < int(buff.size); {
			next := i + int(buff.chunkSize)
			if next > int(buff.size) {
				break
			}
			chunkNum++
			nxt := read_uint32(data[i:])
			idx := uint32(chunkNum)
			ch := Chunk{data[i:next], idx, nxt}
			mpChunck[idx] = ch
			if nxt == 0 { //free
				buff.freeChunk.PushBack(ch)
			} else if idx == nxt { //end of node
				vChunkEnd = append(vChunkEnd, idx)
			} else {
				mpChunkPrev[nxt] = idx
			}
			i = next
		}
		buff.chunkNum = uint32(chunkNum)
		buff.newLRU()

		for _, e := range vChunkEnd {
			node := Node{}
			node.chunks = append(node.chunks, mpChunck[e])
			pre, ok := mpChunkPrev[e]
			for ok {
				p, o := mpChunck[pre]
				if !o {
					return nil, fmt.Errorf("data is valid, pre=%d", pre)
				}
				node.chunks = append([]Chunk{p}, node.chunks...)
				pre, ok = mpChunkPrev[pre]
			}
			node.keySize = read_uint32(node.chunks[0].data[4:])
			node.valSize = read_uint32(node.chunks[0].data[8:])
			if node.keySize+node.valSize > uint32(len(node.chunks))*(buff.chunkSize-4)-8 {
				return nil, fmt.Errorf("data is valid, keysize=%d&valsize=%d", node.keySize, node.valSize)
			}
			key := node.GetKey()
			buff.lru.Add(bytesToStringUnsafe(key), node)
		}

		if buff.size < uint64(len(data)) { //new data
			buff.size = uint64(len(data))
			chunkNum := 0
			for i := HeaderSize; i < int(buff.size); {
				next := i + int(buff.chunkSize)
				if next > int(buff.size) {
					break
				}
				chunkNum++
				if uint32(chunkNum) <= buff.chunkNum {
					i = next
					continue
				}
				write_uint32(data[i:], 0)
				idx := uint32(chunkNum)
				ch := Chunk{data[i:next], idx, 0}
				buff.freeChunk.PushBack(ch)
				i = next
			}
			buff.chunkNum = uint32(chunkNum)
			write_uint64(data[8:], buff.size)
			buff.lru.Resize(int(buff.chunkNum))
		}
	} else {
		if chunkSize < 32 {
			return nil, fmt.Errorf("chunksize min size is 32 , now is %d", chunkSize)
		}
		data[0] = 0xFE
		data[1] = 0xDC
		data[2] = 0xBA
		data[3] = 0x98
		buff.chunkSize = uint32(chunkSize)
		write_uint32(data[4:], buff.chunkSize)
		buff.size = uint64(len(data))
		write_uint64(data[8:], buff.size)

		chunkNum := 0
		for i := HeaderSize; i < int(buff.size); {
			next := i + int(buff.chunkSize)
			if next > int(buff.size) {
				break
			}
			chunkNum++
			write_uint32(data[i:], 0)
			idx := uint32(chunkNum)
			ch := Chunk{data[i:next], idx, 0}
			buff.freeChunk.PushBack(ch)
			i = next
		}
		buff.chunkNum = uint32(chunkNum)
		buff.newLRU()
	}

	return buff, nil
}

func (buff *KVBuffer) newLRU() {
	buff.lru, _ = NewLRU(int(buff.chunkNum), func(key interface{}, value interface{}) {
		//delete key
		if buff.onLruDel != nil {
			buff.onLruDel(key.(string), value.(Node))
		}
		buff.delNode(value.(Node))
	})
}

func (b *KVBuffer) Get(k string) (string, bool) {
	v, ok := b.lru.Get(k)
	if ok {
		node := v.(Node)
		val := node.GetVal()
		return bytesToStringUnsafe(val), true
	}
	return "", false
}

func (b *KVBuffer) Set(key, val string) error {
	need := uint32(len(key) + len(val) + 8)
	if need > (b.chunkSize-4)*b.chunkNum {
		return fmt.Errorf("key+val is too long,size=%d", len(key)+len(val))
	}
	chNeed := need / (b.chunkSize - 4)
	if need%(b.chunkSize-4) > 0 {
		chNeed++
	}

	v, ok := b.lru.Get(key)
	if ok {
		node := v.(Node)
		idx, start := node.getValPos()
		chNow := uint32(len(node.chunks))
		for chNeed > chNow {
			e := b.freeChunk.Front()
			if e == nil {
				if !b.openLru {
					return fmt.Errorf("buffer is full.")
				}
				b.lru.removeOldest()
			} else {
				c := e.Value.(Chunk)
				b.freeChunk.Remove(e)
				node.chunks = append(node.chunks, c)
				chNow++
			}
		}
		for chNeed < chNow {
			chNow--
			b.delChunk(node.chunks[chNow])
			node.chunks = node.chunks[0:chNow]
		}
		if chNeed != chNow {
			panic(fmt.Errorf("set error chNeed:%d != chNow:%d", chNeed, chNow))
		}

		vl := stringToBytesUnsafe(val)
		node.valSize = uint32(len(val))
		write_uint32(node.chunks[0].data[8:], node.valSize)
		writeVLen := node.valSize
		for ; idx < len(node.chunks); idx++ {
			c := node.chunks[idx]
			if idx+1 == len(node.chunks) {
				write_uint32(c.data, c.index)
			} else {
				write_uint32(c.data, node.chunks[idx+1].index)
			}
			if writeVLen > 0 {
				copy(c.data[start:], vl[node.valSize-writeVLen:])
				if writeVLen > b.chunkSize-start {
					writeVLen -= b.chunkSize - start
				}
			}
			start = 4
		}
		b.lru.Add(key, node)
	} else {
		for chNeed > uint32(b.freeChunk.Len()) {
			if !b.openLru {
				return fmt.Errorf("buffer is full.")
			}
			b.lru.removeOldest()
		}

		node := Node{}
		node.keySize = uint32(len(key))
		node.valSize = uint32(len(val))
		for chNeed > 0 {
			e := b.freeChunk.Front()
			c := e.Value.(Chunk)
			b.freeChunk.Remove(e)
			node.chunks = append(node.chunks, c)
			chNeed--
		}
		writeKLen := node.keySize
		writeVLen := node.valSize
		k := stringToBytesUnsafe(key)
		v := stringToBytesUnsafe(val)
		for i, c := range node.chunks {
			var start uint32 = 4
			if i == 0 {
				write_uint32(c.data[4:], node.keySize)
				write_uint32(c.data[8:], node.valSize)
				start += 8
			}
			if i+1 == len(node.chunks) {
				write_uint32(c.data, c.index)
			} else {
				write_uint32(c.data, node.chunks[i+1].index)
			}

			if writeKLen > 0 {
				copy(c.data[start:], k[node.keySize-writeKLen:])
				if writeKLen > b.chunkSize-start {
					writeKLen -= b.chunkSize - start
					start = b.chunkSize
				} else {
					start += writeKLen
					writeKLen = 0
				}
			}

			if writeVLen > 0 && start < b.chunkSize {
				copy(c.data[start:], v[node.valSize-writeVLen:])
				if writeVLen > b.chunkSize-start {
					writeVLen -= b.chunkSize - start
				} else {
					writeVLen = 0
					break
				}
			}
		}
		b.lru.Add(key, node)
	}
	return nil
}

func (b *KVBuffer) Del(k string) bool {
	return b.lru.Remove(k)
}

func (b *KVBuffer) delNode(n Node) {
	for _, c := range n.chunks {
		b.delChunk(c)
	}
}

func (b *KVBuffer) delChunk(c Chunk) {
	write_uint32(c.data, 0)
	c.next = 0
	e := b.freeChunk.Front()
	for e != nil && e.Value.(Chunk).index < c.index {
		e = e.Next()
	}
	if e == nil {
		b.freeChunk.PushBack(c)
	} else {
		b.freeChunk.InsertBefore(c, e)
	}
}

type BufferStat struct {
	FreeChunkNum  int
	UsedChunkNum  int
	FreeDataSize  int
	UsedDataSize  int
	CanotUseSize  int //浪费的空间
	KVCount       int
	AvgKVUseChunk int     //kv平均占用块数
	AvgKVUseSize  int     //kv平均占用空间大小
	AvalidUsePer  float64 //空间有效利用率
}

func (b *KVBuffer) Stat() BufferStat {
	st := BufferStat{}
	st.FreeChunkNum = b.freeChunk.Len()
	st.UsedChunkNum = int(b.chunkNum) - st.FreeChunkNum
	e := b.lru.evictList.Front()
	for e != nil {
		node := e.Value.(*entry).value.(Node)
		used := int(node.keySize+node.valSize) + 8 + len(node.chunks)*4
		st.UsedDataSize += used
		st.CanotUseSize += len(node.chunks)*int(b.chunkSize) - used
		e = e.Next()
	}
	st.FreeDataSize = st.CanotUseSize + st.FreeChunkNum*int(b.chunkNum-4)
	st.KVCount = b.lru.Len()
	if st.KVCount > 0 {
		st.AvgKVUseChunk = st.UsedChunkNum / st.KVCount
		st.AvgKVUseSize = st.UsedDataSize / st.KVCount
	}
	st.AvalidUsePer = float64(st.UsedDataSize) / float64(st.UsedDataSize+st.CanotUseSize) * 100
	return st
}
