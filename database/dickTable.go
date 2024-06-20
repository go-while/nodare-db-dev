package database

import (
	//"hash/fnv"
	"encoding/binary"
	"math"
	"math/bits"
	//"os"
	"log"
	"sync"
)

type DickEntry struct {
	emux  sync.RWMutex
	load  int64
	next  *DickEntry
	prev  *DickEntry
	key   string
	value string
}

type DickTable struct {
	load     int64
	tmux     sync.RWMutex
	tableSLI []*DickEntry
	tableMAP map[string]string
	size     int64
	used     int64
	sizemask uint64
}

func NewDickTable(size int64) (dt *DickTable) {
	dt = &DickTable{}
	switch SYSMODE {
	case MAPMODE:
		dt.tableMAP = make(map[string]string, size)
	case SLIMODE:
		dt.tableSLI = make([]*DickEntry, size)
		dt.sizemask = uint64(size - 1)
		//log.Printf("INFO NewDickTable SLIMODE not yet implemented")
	}
	dt.size = size
	log.Printf("NewDickTable size=%d sizemask=%d", dt.size, dt.sizemask)
	return
} // end func NewDickTable

// https://www.reddit.com/r/golang/comments/xuomty/what_do_you_typically_use_for_noncryptographic/
type V3 struct {
	x, y, z float32
}

func V3hash(v V3) uint64 {
	var buf [8]byte
	binary.LittleEndian.PutUint32(buf[0:], math.Float32bits(v.x))
	binary.LittleEndian.PutUint32(buf[4:], math.Float32bits(v.y))
	h := binary.LittleEndian.Uint64(buf[0:])
	h *= 1111111111111111111
	h ^= h >> 32
	h ^= uint64(math.Float32bits(v.z))
	h *= 1111111111111111111
	return h
}

type slot struct {
	key   V3
	count int
}

type Counter struct {
	exp   int
	slots []slot
}

// New constructs a counter hash table for holding up to n elements.
func New(n int) *Counter {
	exp := 1 + 64 - bits.LeadingZeros64(uint64(n))
	return &Counter{exp, make([]slot, 1<<exp)}
}

// Insert the vector into the table, counting it.
func (c Counter) Insert(v V3) {
	h := V3hash(v)
	mask := int((1 << c.exp) - 1)
	step := int(h>>(64-c.exp) | 1)
	next := int(h)
	for {
		next = (next + step) & mask
		if c.slots[next].count == 0 {
			c.slots[next].key = v
			c.slots[next].count = 1
			break
		} else if c.slots[next].key == v {
			c.slots[next].count++
			break
		}
	}
}

// Lookup the count for a vector.
func (c Counter) Lookup(v V3) int {
	h := V3hash(v)
	mask := int((1 << c.exp) - 1)
	step := int(h>>(64-c.exp) | 1)
	next := int(h)
	for {
		next = (next + step) & mask
		if c.slots[next].count == 0 || c.slots[next].key == v {
			return c.slots[next].count
		}
	}
}
