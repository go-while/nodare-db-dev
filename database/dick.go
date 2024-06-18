package database


import (
	"encoding/binary"
	"fmt"
	"github.com/go-while/nodare-db-dev/logger"
	pcashash "github.com/go-while/nodare-db-dev/pcas_hash"
	"hash/fnv"
	"math/rand"
	"hash/crc32"
	//"strconv"
	"sync"
	"time"
)

var SYSMODE int

const (
	MAPMODE = 1
	SLIMODE = 2
	INITIAL_SIZE = int64(128)
	//MAX_SIZE     = 1 << 63
	//HASH_siphash = 0x01
	HASH_FNV32A  = 0x02
	HASH_FNV64A  = 0x03
)

var AVAIL_SUBDICKS = []int{16, 256, 4096, 65536}

var key0, key1 uint64 // why
var once sync.Once // why
var SALT [16]byte // why

type XDICK struct {
	// mainmux is not used anywhere but exists
	//  for whatever reason we may find
	//   subdicks can lock XDICK
	booted   int64 // timestamp
	mainmux  sync.RWMutex
	SubDICKsSLI []*SubDICK // idi is index to [] holding 10, 100, 1000, 10000 subdicks
	SubDICKsMAP map[string]*SubDICK // key=string=idx
	SubDepth int
	SubCount uint32 // used with SLI
	HashMode int
	logs     ilog.ILOG
	pcas     *pcashash.Hash // hash go objects
}

type SubDICK struct {
	parent     sync.RWMutex
	submux     sync.RWMutex
	dickTable  *DickTable // MAP
	hashTables [2]*DickTable // SLI
	rehashidi  int64 // SLI
	logs       ilog.ILOG
}

func NewXDICK(logs ilog.ILOG, sub_dicks int, hashmode int) *XDICK {
	var mainmux sync.RWMutex
	// create sub_dicks

	xdick := &XDICK{
		pcas: pcashash.New(),
		mainmux:  mainmux,
		SubCount: uint32(sub_dicks),
		HashMode: hashmode, // must not change on runtime
		logs:     logs,
	}


	switch SYSMODE {
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
					parent:     mainmux,
					dickTable: NewDickTable(INITIAL_SIZE),
					//logs:       logs,
				}
				xdick.SubDICKsMAP[idx] = subDICK
			} // end for idx
		/*
		case SLIMODE:
			switch sub_dicks {
				case 100:
					// pass
				case 1000:
					// pass
				case 10000:
					// pass
				default:
					logs.Fatal("invalid sub_dicks=%d! available: 100, 1000, 10000", sub_dicks)
			}
			//xdick.SubDICKsSLI = make([]*SubDICK, sub_dicks)
			for i := 0; i < sub_dicks; i++ {
				subDICK := &SubDICK{
					parent:     mainmux,
					hashTables: [2]*DickTable{NewDickTable(0), NewDickTable(0)},
					rehashidi:  -1,
					//logs:       logs,
				}
				xdick.SubDICKsSLI = append(xdick.SubDICKsSLI, subDICK)
				//xdick.SubDICKsSLI[i] = subDICK
			} // end for
			logs.Debug("Created subDICKs %d/%d ", len(xdick.SubDICKsSLI), sub_dicks)
		*/
	}

	//for idx := range combinations {
	//	go xdick.watchDog(uint32(idx)) // FIXME:::cannot use uint32(idx) (value of type uint32) as string value in argument to xdick.watchDog
	//}

	logs.Debug("Created subDICKs %d/%d ", len(xdick.SubDICKsMAP), sub_dicks)
	return xdick
} // end func NewXDICK

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

func (d *XDICK) keyIndex(key string, idx *string, idi *int64, ind *int64, hashedKey *uint64) () {
	switch SYSMODE {
		case MAPMODE:
			// generate a quick hash and cuts N chars to divide into sub_dicks 0__-f__
			switch d.HashMode {
				case 1:
				// use PCAS
				*idx = fmt.Sprintf("%04x", pcashash.String(key), d.SubDepth)[:d.SubDepth]
				case 2:
				// use CRC32
					generateCRC32AsString(key, d.SubDepth, idx)//[:d.SubDepth]
				case 3:
				// uses FNV1
					generateFNV1aHash(key, d.SubDepth, idx)//[:d.SubDepth]
			}
		/*
		case SLIMODE:
			*idi = int64(pcashash.String(key) % d.SubCount)
			d.expandIfNeeded(*idi)
			hashed := d.SLIhasher(key)
			if hashedKey != nil {
				*hashedKey = hashed
				return
			}
			//var index int
			loops1 := 0
			loops2 := 0
			for i := 0; i <= 1; i++ {
				loops1++
				hashTable := d.SubDICKsSLI[*idi].hashTables[i]
				if hashTable == nil {
					d.logs.Fatal("Keyindex hashTable=nil")
				}
				*ind = int64(hashed & hashTable.sizemask)
				d.logs.Info("keyIndex key='%s' *idi=%d *ind=%d hashed=%d sizemask=%d d.SubCount=%d", key, *idi, *ind, hashed, hashTable.sizemask, d.SubCount)
				d.SubDICKsSLI[*idi].submux.Lock()
				defer d.SubDICKsSLI[*idi].submux.Unlock()
				for entry := hashTable.tableSLI[*ind]; entry != nil; entry = entry.next {
					loops2++
					if entry.key == key {
						//if !overwrite {
							d.logs.Debug("BREAKPOINT keyIndex [%d] entry.key==key='%s' loops1=%d loops2=%d return -1", idi, key, loops1, loops2)
							*ind = -1
							return
						//}
					}
				}

				if *ind == -1 || !d.isRehashing(*idi) {
					//d.logs.Fatal("BREAKPOINT does this ever hit???")
					break
				}
			}
		*/
	} // end switch SYSMODE

	//d.logs.Debug("key=%s idx='%#v' idi='%#v' index='%#v'", key, *idx, *idi, *ind)
} // end func KeyIndex

func (d *XDICK) Set(key string, value string, overwrite bool) bool {
	switch SYSMODE {
		case MAPMODE:
			var idx string
			d.keyIndex(key, &idx, nil, nil, nil)
			d.SubDICKsMAP[idx].submux.Lock()
			defer d.SubDICKsMAP[idx].submux.Unlock()
			if !overwrite {
				if _, containsKey := d.SubDICKsMAP[idx].dickTable.tableMAP[key]; containsKey {
					return false
				}
			}
			d.SubDICKsMAP[idx].dickTable.tableMAP[key] = value
		/*
		case SLIMODE:
			var idi, ind int64
			// TODO
			d.keyIndex(key, nil, &idi, &ind, nil)
			d.expandIfNeeded(idi)
			d.SubDICKsSLI[idi].submux.Lock()
			d.addEntry(idi, ind, key, value, overwrite)
			d.SubDICKsSLI[idi].submux.Unlock()
		*/
	}
	return true
} // end func Set

/*
func (d *XDICK) addEntry(idi int64, ind int64, key string, value string, overwrite bool) bool {
	//d.logs.Debug("addEntry(key=%d='%s' value='%#v' X=%d", len(key), key, value, index)

	if ind == -1 {
		d.logs.Fatal(`addEntry unexpectedly found an entry with the same key when trying to add #{ %s } / #{ %s }`, key, value)
	}

	hashTable := d.mainDICK(idi)
	if d.isRehashing(idi) {
		d.rehashStep(idi)
		hashTable = d.mainDICK(idi)
		if d.isRehashing(idi) {
			hashTable = d.rehashingTable(idi)
		}
	}

	entry := hashTable.tableSLI[ind]

	for entry != nil && entry.key != key {
		entry = entry.next
	}

	if entry == nil {
		entry = NewDickEntry(key, value)
		entry.next = hashTable.tableSLI[ind]
		hashTable.tableSLI[ind] = entry
		hashTable.used++
		return true
	}

	return false
} // end func addEntry
*/

func (d *XDICK) Get(key string, val *string) (containsKey bool) {
	switch SYSMODE {
		case MAPMODE:
			var idx string
			d.keyIndex(key, &idx, nil, nil, nil)
			d.logs.Info("MAP GET getEntry key='%s' idx=%s", key, idx)
			d.SubDICKsMAP[idx].submux.RLock()
			if _, containsKey = d.SubDICKsMAP[idx].dickTable.tableMAP[key]; containsKey {
				*val = d.SubDICKsMAP[idx].dickTable.tableMAP[key]
			}
			d.SubDICKsMAP[idx].submux.RUnlock()
		/*
		case SLIMODE:
			var idi int64
			var hk uint64
			// TODO
			d.keyIndex(key, nil, &idi, nil, &hk)
			d.logs.Info("SLI GET getEntry key='%s' idi=%d hk=%d", key, idi, hk)
			d.SubDICKsSLI[idi].submux.RLock()
			entry := d.getEntry(idi, hk, key)
			if entry == nil {
				d.SubDICKsSLI[idi].submux.RUnlock()
				return
			}
			*val = entry.value
			d.SubDICKsSLI[idi].submux.RUnlock()
		*/
	}
	return
} // end func Get
/*
func (d *XDICK) getEntry(idi int64, hashedKey uint64, key string) *DickEntry {
	d.logs.Info("join SLI getEntry key='%s'", key)

	if d.mainDICK(idi).used == 0 && d.rehashingTable(idi).used == 0 {
		d.logs.Info("SLI getEntry key='%s' return: used is 0", key)
		return nil
	}

	//hashedKey := d.SLIhasher(key) // TODO!FIXME: hash earlier?


	for i, hashTable := range []*DickTable{d.mainDICK(idi), d.rehashingTable(idi)} {
		if hashTable == nil || len(hashTable.tableSLI) == 0 || (i == 1 && !d.isRehashing(idi)) {
			continue
		}
		if hashTable.tableSLI == nil {
			d.logs.Fatal("SLI getEntry hashTable.tableSLI == nil")
		}
		index := int64(hashedKey & hashTable.sizemask)
		d.logs.Info("SLI getEntry hashTable.tableSLI=%d idi=%d hk=%d key='%s' index=%d", len(hashTable.tableSLI), idi, hashedKey, key, index)

		if hashTable.tableSLI[index] == nil {
			d.logs.Error("SLI getEntry hashTable.tableSLI=%d idi=%d hk=%d key='%s' hashTable.tableSLI[index=%d]==nil", len(hashTable.tableSLI), idi, hashedKey, key, index)
			return nil
		}

		entry := hashTable.tableSLI[index]

		for entry != nil {
			if entry.key == key {
				return entry
			}
			entry = entry.next
		}
	}

	return nil
} // end func getEntry
*/
func (d *XDICK) Del(key string) (containsKey bool) {
	switch SYSMODE {
		case MAPMODE:
			var idx string
			d.keyIndex(key, &idx, nil, nil, nil)
			d.SubDICKsMAP[idx].submux.RLock()
			_, containsKey = d.SubDICKsMAP[idx].dickTable.tableMAP[key]
			d.SubDICKsMAP[idx].submux.RUnlock()
			if containsKey {
				d.SubDICKsMAP[idx].submux.Lock()
				delete(d.SubDICKsMAP[idx].dickTable.tableMAP, key)
				d.SubDICKsMAP[idx].submux.Unlock()
			}
		/*
		case SLIMODE:
			var idi, ind int64
			// TODO
			d.keyIndex(key, nil, &idi, &ind, nil)
			containsKey =  d.delEntry(idi, ind, key)
		*/
	}
	return
} // end func Del

/*
func (d *XDICK) delEntry(idi int64, index int64, key string) (containsKey bool) {

	if d.mainDICK(idi).used == 0 && d.rehashingTable(idi).used == 0 {
		return
	}

	if d.isRehashing(idi) {
		d.rehashStep(idi)
	}

	//hashedKey := d.SLIhasher(key) // TODO!FIXME: hash earlier!

	for i, hashTable := range []*DickTable{d.mainDICK(idi), d.rehashingTable(idi)} {
		if hashTable == nil || (i == 1 && !d.isRehashing(idi)) {
			continue
		}
		//index := hashedKey & hashTable.sizemask
		entry := hashTable.tableSLI[index]
		var previousEntry *DickEntry

		for entry != nil {
			if entry.key == key {
				if previousEntry != nil {
					previousEntry.next = entry.next
				} else {
					hashTable.tableSLI[index] = entry.next
				}
				hashTable.used--
				containsKey = true
				return
			}
			previousEntry = entry
			entry = entry.next
		}
	}

	return
} // end func delEntry
*/

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

// GenerateSALT generates a fixed slice of 16 maybe NOT really random bytes
func (d *XDICK) GenerateSALT() {
	once.Do(func() {
		rand.Seed(time.Now().UnixNano())
		cs := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
		for i := 0; i < 16; i++ {
			SALT[i] = cs[rand.Intn(len(cs))]
		}
		key0, key1 = d.SipHashSplit(SALT)
	})
}

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

func (d *XDICK) SipHashSplit(key [16]byte) (uint64, uint64) {
	if len(key) == 0 || len(key) < 16 {
		d.logs.Error("ERROR split len(key)=%d", len(key))
		return 0, 0
	}
	key0 := binary.LittleEndian.Uint64(key[:8])
	key1 := binary.LittleEndian.Uint64(key[8:16])
	return key0, key1
}
