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
	load     uint64
	tmux     sync.RWMutex
	//tableSLI []*Buckets
	tableSLI []*DickEntry  // hashedKey
	tableMAP map[string]*MapEntry
	size     uint32
	used     uint64
	sizemask uint32
}

type Buckets []*Bucket
type Bucket []*DickEntry

func NewDickTable(size uint32) (dt *DickTable) {
	dt = &DickTable{}
	switch SYSMODE {
	case MAPMODE:
		dt.tableMAP = make(map[string]*MapEntry, size)
	case SLIMODE:
		dt.tableSLI = make([]*DickEntry, size)
		/*
		dt.tableSLI = make([]*Buckets, size)
		for i := 0; i < size; i++ {
			dt.tableSLI[i] = make([]*Bucket, size)
		}
		*/
		dt.sizemask = size - 1
	}
	dt.size = size
	log.Printf("NewDickTable size=%d sizemask=%d", dt.size, dt.sizemask)
	return
} // end func NewDickTable
