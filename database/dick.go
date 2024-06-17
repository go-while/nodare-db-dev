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

const (
	INITIAL_SIZE = 1024*1024
	HASH_siphash = 0x01
	HASH_FNV32A  = 0x02
	HASH_FNV64A  = 0x03
)

var AVAIL_SUBDICKS = []int{16, 256, 4096}

var key0, key1 uint64 // why
var once sync.Once // why
var SALT [16]byte // why

type XDICK struct {
	// mainmux is not used anywhere but exists
	//  for whatever reason we may find
	//   subdicks can lock XDICK
	booted   int64 // timestamp
	mainmux  sync.RWMutex
	SubDICKs map[string]*SubDICK // key=string=idx
	SubDepth int
	HashMode int
	logs     ilog.ILOG
	pcas     *pcashash.Hash // hash go objects
}

type SubDICK struct {
	parent     sync.RWMutex
	submux     sync.RWMutex
	dickTable  *DickTable
	logs       ilog.ILOG
}

func NewXDICK(logs ilog.ILOG, sub_dicks int) *XDICK {
	var mainmux sync.RWMutex
	// create sub_dicks
	depth := 0
	switch sub_dicks {
		case AVAIL_SUBDICKS[0]:
			depth = 1
		case  AVAIL_SUBDICKS[1]:
			depth = 2
		case  AVAIL_SUBDICKS[2]:
			depth = 3
		case  AVAIL_SUBDICKS[3]:
			depth = 4
		default:
			logs.Fatal("sub_dicks can not be 0: 16, 256, 4096, 65536, 1048576, 16777216")
	}
	xdick := &XDICK{
		pcas: pcashash.New(),
		mainmux:  mainmux,
		SubDepth: depth,
		logs:     logs,
	}
	// creates sub_dicks by depth value
	var combinations []string
	generateHexCombinations(depth, "", &combinations)
	xdick.SubDICKs = make(map[string]*SubDICK, len(combinations))
	logs.Debug("Create sub_dicks=%d comb=%d", sub_dicks, len(combinations))
	for _, idx := range combinations {
		subDICK := &SubDICK{
			parent:     mainmux,
			dickTable: NewDickTable(INITIAL_SIZE),
			//logs:       logs,
		}
		xdick.SubDICKs[idx] = subDICK
	} // end for idx

	//for idx := range combinations {
	//	go xdick.watchDog(uint32(idx)) // FIXME:::cannot use uint32(idx) (value of type uint32) as string value in argument to xdick.watchDog
	//}

	logs.Debug("Created subDICKs %d/%d ", len(xdick.SubDICKs), sub_dicks)
	return xdick
}

func generateCRC32AsString(input string, sd int, output *string) {
	byteSlice := []byte(input)
	hash := crc32.NewIEEE()
	hash.Write(byteSlice)
	checksum := hash.Sum32()
	//checksumStr := strconv.FormatUint(uint64(checksum), 16)
	*output = fmt.Sprintf("%04x", checksum)[:sd]
}

func generateFNV1aHash(input string, sd int, output *string) {
	hash := fnv.New32a()
	hash.Write([]byte(input))
	hashValue := hash.Sum32()
	//hashValueStr := strconv.FormatUint(uint64(hashValue), 16)
	*output = fmt.Sprintf("%04x", hashValue)[:sd]
}

func (d *XDICK) KeyIndex(key string, idx *string) {
	d.HashMode = 1
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
	d.logs.Debug("key=%s idx='%#v'", key, *idx)
}

func (d *XDICK) set(key string, value string, overwrite bool) bool {
	var idx string
	d.KeyIndex(key, &idx)
	d.SubDICKs[idx].submux.Lock()
	defer d.SubDICKs[idx].submux.Unlock()
	if !overwrite {
		if _, containsKey := d.SubDICKs[idx].dickTable.table[key]; containsKey {
			return false
		}
	}
	d.SubDICKs[idx].dickTable.table[key] = value
	return true
} // end func set

func (d *XDICK) get(key string, val *string) (containsKey bool) {
	var idx string
	d.KeyIndex(key, &idx)
	d.SubDICKs[idx].submux.RLock()
	if _, containsKey = d.SubDICKs[idx].dickTable.table[key]; containsKey {
		*val = d.SubDICKs[idx].dickTable.table[key]
	}
	d.SubDICKs[idx].submux.RUnlock()
	return
} // end func get

func (d *XDICK) del(key string) bool {
	var idx string
	d.KeyIndex(key, &idx)
	d.SubDICKs[idx].submux.RLock()
	_, containsKey := d.SubDICKs[idx].dickTable.table[key]
	d.SubDICKs[idx].submux.RUnlock()

	if containsKey {
		d.SubDICKs[idx].submux.Lock()
		delete(d.SubDICKs[idx].dickTable.table, key)
		d.SubDICKs[idx].submux.Unlock()
	}
	return containsKey
} // end func del

func (d *XDICK) watchDog(idx string) {
	//log.Printf("Booted Watchdog [%s]", idx)
	return
	for {
		time.Sleep(time.Second) // TODO!FIXME setting: watchdog_timer

		if d == nil || d.SubDepth == 0 || d.SubDICKs[idx] == nil {
			// not finished booting
			continue
		}

		if !d.SubDICKs[idx].logs.IfDebug() {
			time.Sleep(59 * time.Second)
			continue
		}

		// print some statistics
		d.SubDICKs[idx].submux.RLock()
		//ht := len(d.SubDICKs[idx].dickTable), // is always 2
		ht0 := d.SubDICKs[idx].dickTable.used
		if ht0 == 0 {
			// subdick is empty
			d.SubDICKs[idx].submux.RUnlock()
			continue
		}
		ht0cap := len(d.SubDICKs[idx].dickTable.table)
		//ht1cap := len(d.SubDICKs[idx].dickTable[1].table)
		d.logs.Info("watchDog [%d] ht0=%d/%d", idx, ht0, ht0cap)
		d.SubDICKs[idx].submux.RUnlock()
		////d.logs.Debug("watchDog [%d] SubDICKs='\n   ---> %#v", idx, d.SubDICKs[idx])
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
