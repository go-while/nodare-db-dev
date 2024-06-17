package database

type DickTable struct {
	//table    []*DickEntry
	table      map[string]string
	size       int64
	used       int64
}

func NewDickTable(size int64) *DickTable {
	return &DickTable{
		table:    make(map[string]string, size),
		size:     size,
	}
}

func (ht *DickTable) empty() bool {
	return ht.size == 0
}
