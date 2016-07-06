package hash

type genericHashMapEntry struct {
	key   uint
	value interface{}
}

type genericHashMapBucket struct {
	bits    uint
	count   uint
	entries [entriesPerHashBucket]genericHashMapEntry
}

type GenericHashMapIterator struct {
	m                               *GenericHashMap
	started                         bool
	curBucketIndex, curElementIndex int
}

func (it *GenericHashMapIterator) Reset() {
	it.started = false
}

func (it *GenericHashMapIterator) Next() bool {
	if !it.started {
		it.started = true
		it.curBucketIndex = 0
		it.curElementIndex = -1
		if it.m.zeroEntryAssigned {
			return true
		}
	}
	if it.curBucketIndex == len(it.m.dir) {
		return false
	}
	for {
		it.curElementIndex++
		if it.curElementIndex == entriesPerHashBucket {
			it.curElementIndex = 0
			for {
				it.curBucketIndex++
				if it.curBucketIndex == len(it.m.dir) {
					return false
				}
				if it.m.dir[it.curBucketIndex] != it.m.dir[it.curBucketIndex-1] {
					break
				}
			}
		}
		if it.m.dir[it.curBucketIndex].entries[it.curElementIndex].key != 0 {
			return true
		}
	}
}
func (it *GenericHashMapIterator) CurKey() uint {
	if !it.started {
		panic("accessing unstarted iterator")
	}
	if it.curElementIndex == -1 {
		return 0 // zero entry key
	}
	return it.m.dir[it.curBucketIndex].entries[it.curElementIndex].key
}
func (it *GenericHashMapIterator) Cur() interface{} {
	if !it.started {
		panic("accessing unstarted iter")
	}
	if it.curElementIndex == -1 {
		return it.m.zeroEntry.value
	}
	return it.m.dir[it.curBucketIndex].entries[it.curElementIndex].value
}

type GenericHashMap struct {
	dirBits           uint
	zeroEntry         genericHashMapEntry
	zeroEntryAssigned bool
	dir               []*genericHashMapBucket
	count             uint
}

func NewMap() *GenericHashMap {
	m := &GenericHashMap{}
	m.init(defaultHashDirBits)

	return m
}

func (m *GenericHashMap) init(bits uint) {
	initSize := 1 << bits
	m.dirBits = bits
	m.dir = make([]*genericHashMapBucket, initSize)
	m.count = 0
	m.zeroEntryAssigned = false

	firstBucket := &genericHashMapBucket{}

	for i := 0; i < initSize; i++ {
		m.dir[i] = firstBucket
	}
}

func (m *GenericHashMap) Clear() {
	m.init(defaultHashDirBits)
}

// find value for key
func (m *GenericHashMap) Iterator() GenericHashMapIterator {
	return GenericHashMapIterator{m: m}
}
func (m *GenericHashMap) Get(key uint) (interface{}, bool) {
	e := m.find(key, false)
	if e == nil {
		return 0, false
	}
	return e.value, true
}
func (m *GenericHashMap) Put(key uint, value interface{}) {
	m.find(key, true).value = value
}
func (m *GenericHashMap) Exists(key uint) bool {
	return m.find(key, false) != nil
}
func (m *GenericHashMap) Delete(key uint) {
	if key == 0 {
		if m.zeroEntryAssigned {
			m.zeroEntryAssigned = false
			m.count--
		}
	} else {
		panic("implement Delete")
	}
}
func (m *GenericHashMap) Len() uint {
	return m.count
}

func (m *GenericHashMap) split(key uint) {
	h := uintHashCode(key)

	for {
		dirIndex := h >> (bitsPerHashCode - m.dirBits)
		splitBucket := m.dir[dirIndex]
		if splitBucket.count < entriesPerHashBucket {
			return // successfully splitted
		}
		newBits := splitBucket.bits + 1

		workBuckets := [2]*genericHashMapBucket{
			&genericHashMapBucket{bits: newBits},
			&genericHashMapBucket{bits: newBits}}

		if m.dirBits == splitBucket.bits {
			// grow directory
			newDirSize := len(m.dir) * 2
			newDir := make([]*genericHashMapBucket, newDirSize)
			for index, b := range m.dir {
				newDir[2*index] = b
				newDir[2*index+1] = b
			}
			m.dirBits = newBits
			m.dir = newDir
			dirIndex *= 2
		}

		// Copy all elements from split bucket into the new buckets
		for index := 0; index < entriesPerHashBucket; index++ {
			hash := uintHashCode(splitBucket.entries[index].key)
			sel := (hash >> (bitsPerHashCode - newBits)) & 1
			bp := workBuckets[sel]
			elemLoc := hash % entriesPerHashBucket
			for ; bp.entries[elemLoc].key != 0; elemLoc = (elemLoc + 1) % entriesPerHashBucket {
			}
			bp.entries[elemLoc] = splitBucket.entries[index]
			bp.count++
		}

		// replace splitBucket with first work bucket
		dirIndex = h >> (bitsPerHashCode - m.dirBits)
		for {
			if dirIndex == 0 || m.dir[dirIndex-1] != splitBucket {
				break
			}
			dirIndex--
		}
		for i := dirIndex; i < uint(len(m.dir)); i++ {
			if m.dir[i] != splitBucket {
				break
			}
			m.dir[i] = workBuckets[0]
		}

		// update the directory with second work bucket
		dirStart := (dirIndex >> (m.dirBits - newBits)) | 1
		dirEnd := (dirStart + 1) << (m.dirBits - newBits)
		dirStart = dirStart << (m.dirBits - newBits)

		for index := dirStart; index < dirEnd; index++ {
			m.dir[index] = workBuckets[1]
		}
	}
}

// add entry for key (or reuse existing)
func (m *GenericHashMap) find(key uint, addIfNotExists bool) *genericHashMapEntry {
	if key == 0 {
		if !m.zeroEntryAssigned && addIfNotExists {
			m.zeroEntryAssigned = true
			m.count++
		}
		if m.zeroEntryAssigned {
			return &m.zeroEntry
		}
		return nil
	}
	h := uintHashCode(key)
	dirIndex := h >> (bitsPerHashCode - m.dirBits)
	elementIndex := h % entriesPerHashBucket
	b := m.dir[dirIndex]
	homeIndex := elementIndex
	for {
		if b.entries[elementIndex].key == key {
			return &b.entries[elementIndex]
		}
		if b.entries[elementIndex].key == 0 {
			break
		}
		elementIndex = (elementIndex + 1) % entriesPerHashBucket
		if elementIndex == homeIndex {
			break
		}
	}
	// element not found
	if !addIfNotExists {
		return nil
	}
	if b.count == entriesPerHashBucket {
		m.split(key)
		dirIndex = h >> (bitsPerHashCode - m.dirBits)
		b = m.dir[dirIndex]
		elementIndex = h % entriesPerHashBucket
		for ; b.entries[elementIndex].key != 0; elementIndex = (elementIndex + 1) % entriesPerHashBucket {
		}
	}
	b.count++
	b.entries[elementIndex].key = key
	m.count++
	return &b.entries[elementIndex]
}
