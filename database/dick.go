package database


import (
	"fmt"
	"github.com/go-while/nodare-db-dev/logger"
	pcashash "github.com/go-while/nodare-db-dev/pcas_hash"
	"hash/fnv"
	"hash/crc32"
	//"strconv"
	"sync"
	"time"
)

var SYSMODE int

const (
	INITIAL_SIZE = int64(1024*1024)
	MAX_SIZE     = 1 << 63
	MAPMODE = 1
	SLIMODE = 2
	HASH_siphash = 0x01
	HASH_FNV32A  = 0x02
	HASH_FNV64A  = 0x03
)

var AVAIL_SUBDICKS = []int{16, 256, 4096, 65536}

type XDICK struct {
	// mainmux is not used anywhere but exists
	//  for whatever reason we may find
	//   subdicks can lock XDICK
	booted   int64 // timestamp
	mainmux  sync.RWMutex
	SubDICKsMAP map[string]*SubDICK // key=string=idx
	//SubDICKsSLI []*SubDICK
	SubDepth int
	SubCount int
	HashMode int
	logs     ilog.ILOG
	pcas     *pcashash.Hash // hash go objects
}

type SubDICK struct {
	parent     sync.RWMutex
	submux     sync.RWMutex
	dickTable  *DickTable // MAP
	logs       ilog.ILOG
}

func NewXDICK(logs ilog.ILOG, sub_dicks int, hashmode int) *XDICK {
	var mainmux sync.RWMutex
	// create sub_dicks

	xdick := &XDICK{
		pcas: pcashash.New(),
		mainmux:  mainmux,
		SubCount: sub_dicks,
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

		case SLIMODE:
			d.logs.Fatal("not implemented")
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

		case SLIMODE:
			d.logs.Fatal("not implemented")

	}
	return true
} // end func Set

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
		case SLIMODE:
			d.logs.Fatal("not implemented")
	}
	return
} // end func Get

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

		case SLIMODE:
			d.logs.Fatal("not implemented")

	}
	return
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
