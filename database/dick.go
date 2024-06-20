package database

import (
	"encoding/binary"
	"fmt"
	xxhash "github.com/cespare/xxhash"
	"github.com/dchest/siphash"
	"github.com/go-while/nodare-db-dev/logger"
	pcashash "github.com/go-while/nodare-db-dev/pcas_hash"
	"hash/fnv"
	"math/rand"
	//crand "crypto/rand"
	"hash/crc32"
	//"strconv"
	"sync"
	"time"
)

const (
	INITIAL_SIZE = int64(1024 * 1024)
	MAX_SIZE     = 1 << 63
	MAPMODE      = 1
	SLIMODE      = 2
	HASH_siphash = 0x01
	HASH_FNV32A  = 0x02
	HASH_FNV64A  = 0x03
	HASH_XXHASH  = 0x04
	HASH_PCAS    = 0x05
)

var (
	once           sync.Once
	AVAIL_SUBDICKS = []int{16, 256, 4096, 65536, 10, 100, 1000, 10000}
	AVAIL_HASHALGO = []int{HASH_siphash, HASH_FNV32A, HASH_FNV64A, HASH_XXHASH, HASH_PCAS}
	SYSMODE        int
	HASHER         = HASH_XXHASH
	key0, key1     uint64
	SALT           [16]byte
)

type XDICK struct {
	// mainmux is not used anywhere but exists
	//  for whatever reason we may find
	//   subdicks can lock XDICK
	booted      int64 // timestamp
	mainmux     sync.RWMutex
	SubDICKsMAP map[string]*SubDICK // key=string=idx
	SubDICKsSLI []*SubDICK          // key=int=idi
	SubDepth    int
	SubCount    uint64
	HashMode    int
	logs        ilog.ILOG
	pcas        *pcashash.Hash // hash go objects
}

type SubDICK struct {
	parent    *XDICK
	submux    sync.RWMutex
	dickTable *DickTable
	//sizemask   uint64
	logs ilog.ILOG
}

func NewXDICK(logs ilog.ILOG, sub_dicks int, hashmode int) *XDICK {
	var mainmux sync.RWMutex

	xdick := &XDICK{
		pcas:     pcashash.New(),
		mainmux:  mainmux,
		SubCount: uint64(sub_dicks),
		HashMode: hashmode, // must not change on runtime
		logs:     logs,
	}

	// create sub_dicks
	switch SYSMODE {
	case SLIMODE:
		switch sub_dicks {
		case AVAIL_SUBDICKS[4]: // 10
			// pass
		case AVAIL_SUBDICKS[5]: // 100
			// pass
		case AVAIL_SUBDICKS[6]: // 1000
			// pass
		case AVAIL_SUBDICKS[7]: // 10000
			// pass
		default:
			logs.Fatal("invalid sub_dicks=%d! available: 10, 100, 1000, 10000", sub_dicks)
		}
		xdick.SubDICKsSLI = make([]*SubDICK, xdick.SubCount)
		for idi := uint64(0); idi < xdick.SubCount; idi++ {
			subDICK := &SubDICK{
				parent:    xdick,
				dickTable: NewDickTable(INITIAL_SIZE),
				//logs:       logs,
			}
			xdick.SubDICKsSLI[idi] = subDICK
		}
	case MAPMODE:
		switch sub_dicks {
		case AVAIL_SUBDICKS[0]: // 16
			xdick.SubDepth = 1
		case AVAIL_SUBDICKS[1]: // 256
			xdick.SubDepth = 2
		case AVAIL_SUBDICKS[2]: // 4096
			xdick.SubDepth = 3
		case AVAIL_SUBDICKS[3]: // 65536
			xdick.SubDepth = 4
		default:
			logs.Fatal("invalid sub_dicks=%d! available: 16, 256, 4096, 65536", sub_dicks)
		}
		// creates sub_dicks by depth value
		var combinations []string
		generateHexCombinations(xdick.SubDepth, "", &combinations)
		xdick.SubDICKsMAP = make(map[string]*SubDICK, len(combinations))
		logs.Debug("Create sub_dicks=%d comb=%d", sub_dicks, len(combinations))
		for _, idx := range combinations {
			subDICK := &SubDICK{
				parent:    xdick,
				dickTable: NewDickTable(INITIAL_SIZE),
				//logs:       logs,
			}
			xdick.SubDICKsMAP[idx] = subDICK
		} // end for idx
	}

	//for idx := range combinations {
	//	go xdick.watchDog(uint32(idx)) // FIXME:::cannot use uint32(idx) (value of type uint32) as string value in argument to xdick.watchDog
	//}

	logs.Debug("Created subDICKs %d/%d ", len(xdick.SubDICKsMAP), sub_dicks)
	return xdick
} // end func NewXDICK

func (d *XDICK) keyIndex(key string, idx *string, idi *int64, ind *int64, hk *uint64) {
	switch SYSMODE {
	case MAPMODE:
		// generate a quick hash and cuts N chars to divide into sub_dicks 0__-f__
		switch d.HashMode {
		case 1:
			// use PCAS
			*idx = fmt.Sprintf("%04x", pcashash.String(key), d.SubDepth)[:d.SubDepth]
		case 2:
			// use CRC32
			generateCRC32AsString(key, d.SubDepth, idx) //[:d.SubDepth]
		case 3:
			// uses FNV1
			generateFNV1aHash(key, d.SubDepth, idx) //[:d.SubDept
		default:
			d.logs.Fatal("Invalid HashMode")
		}

	case SLIMODE:
		hashedKey := d.hasher(key)
		if hk != nil {
			*hk = hashedKey
		}
		*idi = int64(hashedKey % d.SubCount) // last digits
		d.SubDICKsSLI[*idi].submux.RLock()
		*ind = int64(hashedKey & d.SubDICKsSLI[*idi].dickTable.sizemask)
		d.SubDICKsSLI[*idi].submux.RUnlock()
	} // end switch SYSMODE

	//d.logs.Debug("key=%s idx='%#v' idi='%#v' index='%#v'", key, *idx, *idi, *ind)
} // end func KeyIndex

// main hasher func
func (d *XDICK) hasher(key string) uint64 {
	switch d.HashMode {

	case HASH_siphash:
		//sipHashDigest calculates the SipHash-2-4 digest of the given message using the provided key.
		return siphash.Hash(key0, key1, []byte(key))

	case HASH_FNV32A:
		hash := fnv.New32a()
		hash.Write([]byte(key))
		return uint64(hash.Sum32())

	case HASH_FNV64A:
		hash := fnv.New64a()
		hash.Write([]byte(key))
		return hash.Sum64()

	case HASH_XXHASH:
		return xxhash.Sum64String(key)

	case HASH_PCAS:
		return uint64(pcashash.String(key))

	default:
		d.logs.Fatal("Invalid HashMode")
	}
	d.logs.Fatal("No HASHER defined! HASHER=%d %x", HASHER, HASHER)
	return 0
} // end func hasher

func (d *XDICK) Set(key string, value string, overwrite bool) bool {
	switch SYSMODE {
	case MAPMODE:
		var idx string
		d.keyIndex(key, &idx, nil, nil, nil)
		d.SubDICKsMAP[idx].dickTable.tmux.Lock()
		defer d.SubDICKsMAP[idx].dickTable.tmux.Unlock()
		if !overwrite {
			if _, containsKey := d.SubDICKsMAP[idx].dickTable.tableMAP[key]; containsKey {
				return false
			}
		}
		d.SubDICKsMAP[idx].dickTable.tableMAP[key] = value
		return true

	case SLIMODE:
		var idi, ind int64
		var hashedKey uint64
		d.keyIndex(key, nil, &idi, &ind, &hashedKey)
		d.logs.Debug("SLIMODE Set(key='%s' idi=%d ind=%d hashedKey=%d", key, idi, ind, hashedKey)
		exists, added := d.SetEntry(idi, ind, key, value, overwrite)
		if overwrite && exists {
			return false
		}
		return added
		//if d.EntryExists(idi, ind) {
		//	d.logs.Info("AddEntry => Set(key='%s' idi=%d ind=%d hashedKey=%d", key, idi, ind, hashedKey)
		//}

	}
	d.logs.Warn("uncatched return Set()")
	return false
} // end func Set

func (d *XDICK) SetEntry(idi int64, ind int64, key string, value string, overwrite bool) (exists bool, added bool) {

	d.logs.Debug("SetEntry [%d:%d] k='%v' v='%v'", idi, ind, key, value)

	if d.GetEntry(idi, ind, true) == nil {
		// create new entry
		d.SubDICKsSLI[idi].dickTable.tmux.Lock()
		if d.SubDICKsSLI[idi].dickTable.tableSLI[ind] == nil {
			d.SubDICKsSLI[idi].dickTable.tableSLI[ind] = &DickEntry{key: key, value: value}
			d.SubDICKsSLI[idi].dickTable.tmux.Unlock()
			added = true
			d.usedInc(idi)
			d.logs.Debug("SetEntry [%d:%d] NEW key='%s' exists=%t overwrite=%t added=%t used=%d", idi, ind, key, exists, overwrite, added, d.SubDICKsSLI[idi].dickTable.used)
			return
		}
		d.SubDICKsSLI[idi].dickTable.tmux.Unlock()
	}

	var load int64
	// find entries matching our key to set new value
	var prev *DickEntry = nil
	for entry := d.GetEntry(idi, ind, true); entry != nil; entry = entry.next {
		entry.emux.RLock()
		if entry.key == "" {
			d.logs.Fatal("GetEntry entry.key empty?!")
		}
		if entry.key != key {
			load++
			prev = entry
			entry.emux.RUnlock()
			continue
		}
		entry.emux.RUnlock()
		// got match
		if !overwrite {
			exists = true
			d.logs.Debug("SetEntry [%d:%d] RET load=%d key='%s' exists=%t overwrite=%t added=%t", idi, ind, load, key, exists, overwrite, added)
			return
		}
		entry.emux.Lock()
		// update to new value
		entry.value = value
		entry.emux.Unlock()
		added = true
		d.logs.Debug("SetEntry [%d:%d] SET load=%d key='%s' value='%s' exists=%t overwrite=%t added=%t", idi, ind, load, key, value, exists, overwrite, added)
		return
	} // end for

	// did not find an entry matching our key
	// adds new entry as next to prev and set this as prev.next and prev as next.prev ^^
	if prev != nil && prev.next == nil {
		prev.next = &DickEntry{key: key, value: value, prev: prev}
		d.setEntryLoad(idi, ind, load)
		added = true
		d.usedInc(idi)
		d.logs.Debug("SetEntry [%d:%d] ADD load=%d key='%s' exists=%t overwrite=%t added=%t used=%d", idi, ind, load, key, exists, overwrite, added, d.SubDICKsSLI[idi].dickTable.used)
		return
	}

	// should never reach here
	d.logs.Fatal("SetEntry [%d:%d] ERR load=%d key='%s' exists=%t overwrite=%t added=%t used=%d prev=nil?!", idi, ind, load, key, exists, overwrite, added, d.SubDICKsSLI[idi].dickTable.used)
	return
} // end func SetEntry

func (d *XDICK) Get(key string, val *string) (containsKey bool) {
	switch SYSMODE {
	case MAPMODE:
		var idx string
		d.keyIndex(key, &idx, nil, nil, nil)
		d.logs.Debug("MAP GET getEntry key='%s' idx=%s", key, idx)
		d.SubDICKsMAP[idx].submux.RLock()
		if _, containsKey = d.SubDICKsMAP[idx].dickTable.tableMAP[key]; containsKey {
			*val = d.SubDICKsMAP[idx].dickTable.tableMAP[key]
		}
		d.SubDICKsMAP[idx].submux.RUnlock()
	case SLIMODE:
		var idi, ind int64
		//var hashedKey uint64
		d.keyIndex(key, nil, &idi, &ind, nil)
		//loops := 0
		for entry := d.GetEntry(idi, ind, true); entry != nil; entry = entry.next {
			entry.emux.RLock()
			if entry.key == key {
				*val = entry.value
				entry.emux.RUnlock()
				//if loops > 0 {
				//	d.logs.Debug("SLI GET getEntry [%d:%d] key='%s' loops=%d", idi, ind, key, loops)
				//}
				return true
			}
			entry.emux.RUnlock()
			//loops++
		} // end for entry
	}
	return
} // end func Get

func (d *XDICK) Del(key string) bool {
	switch SYSMODE {
	case MAPMODE:
		var idx string
		d.keyIndex(key, &idx, nil, nil, nil)
		d.SubDICKsMAP[idx].submux.RLock()
		_, containsKey := d.SubDICKsMAP[idx].dickTable.tableMAP[key]
		d.SubDICKsMAP[idx].submux.RUnlock()
		if containsKey {
			d.SubDICKsMAP[idx].submux.Lock()
			delete(d.SubDICKsMAP[idx].dickTable.tableMAP, key)
			d.SubDICKsMAP[idx].submux.Unlock()
			return true
		}

	case SLIMODE:
		var idi, ind int64
		//var hashedKey uint64
		d.keyIndex(key, nil, &idi, &ind, nil)
		d.logs.Info("DelEntry key='%s' idi=%d ind=%d", key, idi, ind)
		d.SubDICKsSLI[idi].dickTable.tmux.Lock()
		defer d.SubDICKsSLI[idi].dickTable.tmux.Unlock()

		for entry := d.GetEntry(idi, ind, false); entry != nil; entry = entry.next {
			if entry.key != key {
				continue
			}
			// found our key to delete
			// this entry has NO next entry and NO prev entry
			if entry.next == nil && entry.prev == nil {
				d.logs.Info("DelEntry1 key='%s'", key)
				d.SubDICKsSLI[idi].dickTable.tableSLI[ind] = nil
				d.usedDec(idi)
				return true
			}

			if entry.next != nil {
				// this entry has a next entry
				if entry.prev == nil {
					// and no prev entry
					entry.next.prev = nil // nil prev-pointer in entry.next.
					// clear k:v
					d.SubDICKsSLI[idi].dickTable.tableSLI[ind].key = ""
					d.SubDICKsSLI[idi].dickTable.tableSLI[ind].value = ""
					// overwrite SLI[ind] with entry.next
					d.SubDICKsSLI[idi].dickTable.tableSLI[ind] = entry.next
					d.usedDec(idi)
					d.logs.Info("DelEntry2 key='%s'", key)
					return true
				}
				// this entry has a prev entry
				// set prev.next to entry.next, kicks out link to actual key
				d.SubDICKsSLI[idi].dickTable.tableSLI[ind].prev.next = entry.next
				d.usedDec(idi)
				d.logs.Info("DelEntry3 key='%s'", key)
				return true
			}

			if entry.prev != nil {
				// entry has a prev entry
				d.logs.Info("DelEntry4 key='%s'", key)
				entry.prev.emux.Lock()
				// nil the prev.next-pointer
				entry.prev.next = nil
				entry.prev.emux.Unlock()
				// nil the pointer to prev-entry
				entry.prev = nil
				// clear k:v
				entry.value = ""
				entry.key = ""
				d.usedDec(idi)
				return true
			}

		} // end for entry
	} // switch SYSMODE

	d.logs.Fatal("Del() uncatched return")
	return false
} // end func Del

func (d *XDICK) watchDog(idx string) {
	//log.Printf("Booted Watchdog [%s]", idx)
	return
	for {
		time.Sleep(time.Second) // TODO!FIXME setting: watchdog_timer

		if d == nil || d.SubDepth == 0 || d.SubDICKsMAP[idx] == nil {
			// not finished booting
			continue
		}

		if !d.SubDICKsMAP[idx].logs.IfDebug() {
			time.Sleep(59 * time.Second)
			continue
		}

		// print some statistics
		d.SubDICKsMAP[idx].submux.RLock()
		//ht := len(d.SubDICKsMAP[idx].dickTable), // is always 2
		ht0 := d.SubDICKsMAP[idx].dickTable.used
		if ht0 == 0 {
			// subdick is empty
			d.SubDICKsMAP[idx].submux.RUnlock()
			continue
		}
		ht0cap := len(d.SubDICKsMAP[idx].dickTable.tableMAP)
		//ht1cap := len(d.SubDICKsMAP[idx].dickTable[1].table)
		d.logs.Info("watchDog [%d] ht0=%d/%d", idx, ht0, ht0cap)
		d.SubDICKsMAP[idx].submux.RUnlock()
		//d.logs.Debug("watchDog [%d] SubDICKs='\n   ---> %#v", idx, d.SubDICKsMAP[idx])
	}
} // end func watchDog

var HEXCHARS = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c", "d", "e", "f"}

// generateHexCombinations generates all combinations of hex characters up to the given depth.
// It takes the current depth, the current combination being built, and a pointer to a slice to store the results.
func generateHexCombinations(depth int, current string, combinations *[]string) {
	// Base case: if depth is 0, add the current combination to the results
	if depth == 0 {
		*combinations = append(*combinations, current)
		return
	}
	// Recursive case: for each hex character, append it to the current combination and reduce depth by 1
	for _, char := range HEXCHARS {
		generateHexCombinations(depth-1, current+char, combinations)
	}
} // end func generateHexCombinations

func (d *XDICK) DelEntry(idi int64, ind int64) {
	d.SubDICKsSLI[idi].dickTable.tableSLI[ind] = nil
} // end func DelEntry

func (d *XDICK) GetEntry(idi int64, ind int64, doRLock bool) (entry *DickEntry) {
	if doRLock {
		d.SubDICKsSLI[idi].dickTable.tmux.RLock()
	}

	entry = d.SubDICKsSLI[idi].dickTable.tableSLI[ind]

	if doRLock {
		d.SubDICKsSLI[idi].dickTable.tmux.RUnlock()
	}

	return
} // end func GetEntry

func (d *XDICK) EntryExists(idi int64, ind int64) (exists bool) {
	//d.SubDICKsSLI[idi].dickTable.tmux.RLock()
	exists = d.SubDICKsSLI[idi].dickTable.tableSLI[ind] != nil
	//d.SubDICKsSLI[idi].dickTable.tmux.RUnlock()
	return
} // end func EntryExists

func (d *XDICK) setEntryLoad(idi int64, ind int64, load int64) {
	d.SubDICKsSLI[idi].dickTable.tableSLI[ind].load = load
} // end func setLoad of table

func (d *XDICK) getEntryLoad(idi int64, ind int64) (load int64) {
	//d.SubDICKsSLI[idi].dickTable.tmux.RLock()
	load = d.SubDICKsSLI[idi].dickTable.tableSLI[ind].load
	//d.SubDICKsSLI[idi].dickTable.tmux.RUnlock()
	return
} // end func setLoad of table

func (d *XDICK) usedInc(idi int64) {
	d.SubDICKsSLI[idi].dickTable.used++
} // end func usedIncrease

func (d *XDICK) usedDec(idi int64) {
	d.SubDICKsSLI[idi].dickTable.used--
} // end func usedDecrease

// GenerateSALT generates a fixed slice of 16 maybe NOT really random bytes
func (d *XDICK) GenerateSALT() {
	once.Do(func() {
		rand.Seed(time.Now().UnixNano())
		cs := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		for i := 0; i < 16; i++ {
			SALT[i] = cs[rand.Intn(len(cs))]
		}
		key0, key1 = d.split(SALT)
	})
}

func (d *XDICK) split(key [16]byte) (uint64, uint64) {
	if len(key) == 0 || len(key) < 16 {
		d.logs.Error("ERROR split len(key)=%d", len(key))
		return 0, 0
	}
	key0 := binary.LittleEndian.Uint64(key[:8])
	key1 := binary.LittleEndian.Uint64(key[8:16])
	return key0, key1
}

func generateCRC32AsString(input string, sd int, output *string) {
	byteSlice := []byte(input)
	hash := crc32.NewIEEE()
	hash.Write(byteSlice)
	checksum := hash.Sum32()
	*output = fmt.Sprintf("%04x", checksum)[:sd]
}

func generateFNV1aHash(input string, sd int, output *string) {
	hash := fnv.New32a()
	hash.Write([]byte(input))
	hashValue := hash.Sum32()
	*output = fmt.Sprintf("%04x", hashValue)[:sd]
}
