package database

import (
	//"hash/fnv"
	//"os"
	"log"
)

type DickEntry struct {
	next  *DickEntry
	key   string
	value string
}

type DickTable struct {
	tableSLI   []*DickEntry
	tableMAP   map[string]string
	size       int64
	used       int64
	sizemask   uint64
}

func NewDickTable(size int64) (dt *DickTable) {
	dt = &DickTable{}
	switch SYSMODE {
		case MAPMODE:
			dt.tableMAP = make(map[string]string, size)

		case SLIMODE:
			dt.tableSLI = make([]*DickEntry, size)
			if size > 0 {
				dt.sizemask = uint64(size) - 1
			}
	}
	dt.size = size
	log.Printf("NewDickTable size=%d sizemask=%d", dt.size, dt.sizemask)
	return
} // end func NewDickTable

/*
func NewDickEntry(key string, value string) *DickEntry {
	return &DickEntry{
		key:   key,
		value: value,
		next:  nil,
	}
} // end func NewDickEntry


func (ht *DickTable) empty() bool {
	return ht.size == 0
}

func (d *XDICK) mainDICK(idi int64) *DickTable {
	return d.SubDICKsSLI[idi].hashTables[0]
}

func (d *XDICK) isRehashing(idi int64) bool {
	return d.SubDICKsSLI[idi].rehashidi != -1
}

func (d *XDICK) rehashingTable(idi int64) *DickTable {
	return d.SubDICKsSLI[idi].hashTables[1]
}

func (d *XDICK) rehashStep(idi int64) {
	d.rehash(idi, 1)
}

func (d *XDICK) SLIhasher(any string) uint64 {
	HASHER := HASH_FNV64A // hardcoded

	switch HASHER {


	//case HASH_siphash:
	//	// sipHashDigest calculates the SipHash-2-4 digest of the given message using the provided key.
	//	return siphash.Hash(key0, key1, []byte(any))


	case HASH_FNV32A:
		algo := fnv.New32a()
		algo.Write([]byte(any))
		return uint64(algo.Sum32())


	case HASH_FNV64A:
		hash := fnv.New64a()
		hash.Write([]byte(any))
		return hash.Sum64()

	}

	d.logs.Fatal("No HASHER defined! HASHER=%d %x", HASHER, HASHER)
	os.Exit(1)
	return 0
} // end func hasher

// n is the new size of the dictionary.
// Returns 0 if the rehashing is not in progress.
// Returns 1 if the rehashing is in progress.
func (d *XDICK) rehash(idi int64, n int) {
	emptyVisits := n * 10
	if !d.isRehashing(idi) {
		d.logs.Info("rehash return !d.isRehashing(idi=%d)", idi)
		return
	}
	d.logs.Debug("rehash SubDick [%d] used=%d", idi, d.mainDICK(idi).used)
	for n > 0 && d.mainDICK(idi).used != 0 {

		n--

		var entry *DickEntry

		for len(d.mainDICK(idi).tableSLI) == 0 || d.mainDICK(idi).tableSLI[d.SubDICKsSLI[idi].rehashidi] == nil {
			d.SubDICKsSLI[idi].rehashidi++
			emptyVisits--
			if emptyVisits == 0 {
				return
			}
		}

		entry = d.mainDICK(idi).tableSLI[d.SubDICKsSLI[idi].rehashidi]

		for entry != nil {
			nextEntry := entry.next
			X := d.SLIhasher(entry.key) & d.rehashingTable(idi).sizemask

			entry.next = d.rehashingTable(idi).tableSLI[X]
			d.rehashingTable(idi).tableSLI[X] = entry
			d.mainDICK(idi).used--
			d.rehashingTable(idi).used++
			entry = nextEntry
		}

		d.mainDICK(idi).tableSLI[d.SubDICKsSLI[idi].rehashidi] = nil
		d.SubDICKsSLI[idi].rehashidi++
	}

	if d.mainDICK(idi).used == 0 {
		d.SubDICKsSLI[idi].hashTables[0] = d.rehashingTable(idi)
		d.SubDICKsSLI[idi].hashTables[1] = NewDickTable(0)
		d.SubDICKsSLI[idi].rehashidi = -1
		return
	}
} // end func rehash

func (d *XDICK) expandIfNeeded(idi int64) {
	d.SubDICKsSLI[idi].submux.Lock()
	defer d.SubDICKsSLI[idi].submux.Unlock()

	if d.isRehashing(idi) {
		return
	}

	if d.mainDICK(idi) == nil || len(d.mainDICK(idi).tableSLI) == 0 {
		d.expand(idi, INITIAL_SIZE)
	} else if d.mainDICK(idi).used >= d.mainDICK(idi).size {
		newSize := d.mainDICK(idi).used * 2
		d.expand(idi, newSize)
	}
} // end func expandIfNeeded


func (d *XDICK) expand(idi int64, newSize int64) {

	isrehashing := d.isRehashing(idi)
	istablefull := d.mainDICK(idi).used > newSize
	if isrehashing || istablefull {
		d.logs.Debug("SubDick [%d] expand return1 ! (newSize=%d isrehashing=%t istablefull=%t used=%d)", idi, newSize, isrehashing, istablefull, d.mainDICK(idi).used)
		return
	}

	d.logs.Debug("SubDick [%d] expand newSize=%d isrehashing=%t istablefull=%t used=%d", idi, newSize, isrehashing, istablefull, d.mainDICK(idi).used)

	nextSize := nextPower(newSize)
	if d.mainDICK(idi).used >= nextSize {
		return
	}

	newDickTable := NewDickTable(nextSize)

	if d.mainDICK(idi) == nil || len(d.mainDICK(idi).tableSLI) == 0 {
		*d.mainDICK(idi) = *newDickTable
		return
	}

	*d.rehashingTable(idi) = *newDickTable
	d.SubDICKsSLI[idi].rehashidi = 0
} // end func expand

func nextPower(size int64) int64 {
	if size <= INITIAL_SIZE {
		return INITIAL_SIZE
	}

	size--
	size |= size >> 1
	size |= size >> 2
	size |= size >> 4
	size |= size >> 8
	size |= size >> 16
	size |= size >> 32

	return size + 1
} // end func nextPower
*/

/*
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
	h := hash(v)
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
	h := hash(v)
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


*/
