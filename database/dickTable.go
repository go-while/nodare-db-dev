package database

import (
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

type MapEntry struct {
	mmux  sync.RWMutex
	value string
}

type DickTable struct {
	load     int64
	tmux     sync.RWMutex
	tableSLI []*DickEntry
	tableMAP map[string]*MapEntry
	size     int64
	used     int64
	sizemask uint64
}

func NewDickTable(size int64) (dt *DickTable) {
	dt = &DickTable{}
	switch SYSMODE {
	case MAPMODE:
		dt.tableMAP = make(map[string]*MapEntry, size)
	case SLIMODE:
		dt.tableSLI = make([]*DickEntry, size)
		dt.sizemask = uint64(size - 1)
	}
	dt.size = size
	log.Printf("NewDickTable size=%d sizemask=%d", dt.size, dt.sizemask)
	return
} // end func NewDickTable
