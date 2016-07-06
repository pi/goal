package hash

type uintHashMapEntry struct {
	key, value uint
}

type uintHashMapBucket struct {
	bits    uint
	count   uint
	entries [entriesPerHashBucket]uintHashMapEntry
}

type UintHashMapIterator struct {
	m               *UintHashMap
	started         bool
	curBucketIndex  int
	curElementIndex int
}

func (it *UintHashMapIterator) Reset() {
	it.started = false
}

func (it *UintHashMapIterator) Next() bool {
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

// Return current map key. Panic if the iterator has not been started
func (it *UintHashMapIterator) CurKey() uint {
	if !it.started {
		panic("accessing unstarted iterator")
	}
	if it.curElementIndex == -1 {
		return 0 // zero entry key
	}
	return it.m.dir[it.curBucketIndex].entries[it.curElementIndex].key
}

// Return current map value. Panic if the iterator has not been started.
func (it *UintHashMapIterator) Cur() uint {
	if !it.started {
		panic("accessing unstarted iter")
	}
	if it.curElementIndex == -1 {
		return it.m.zeroEntry.value
	}
	return it.m.dir[it.curBucketIndex].entries[it.curElementIndex].value
}

//
// HashMap is uint64->uint64 map
//
type UintHashMap struct {
	dirBits           uint
	zeroEntry         uintHashMapEntry
	zeroEntryAssigned bool
	dir               []*uintHashMapBucket
	count             uint
}

func NewUintHashMap(args ...interface{}) *UintHashMap {
	m := &UintHashMap{}
	initBits := uint(0)
	if len(args) > 1 {
		panic("usage: NewHashMap([sizeHint])")
	}
	if len(args) == 1 {
		var initSize uint
		if v, ok := args[0].(uint); ok {
			initSize = v
		} else if v, ok := args[0].(int); ok {
			initSize = uint(v)
		} else if v, ok := args[0].(uint64); ok {
			initSize = uint(v)
		} else if v, ok := args[0].(int64); ok {
			initSize = uint(v)
		} else {
			panic("expected integer init size")
		}

		for sz := uint(0); sz < initSize; {
			sz *= 2
			initBits *= 2
		}
	}
	if initBits < 2 {
		initBits = defaultHashDirBits
	} else if initBits > 62 {
		initBits = 62
	}
	m.init(initBits)

	return m
}

func (m *UintHashMap) init(bits uint) {
	initSize := 1 << bits
	m.dirBits = bits
	m.dir = make([]*uintHashMapBucket, initSize)
	m.count = 0
	m.zeroEntryAssigned = false

	firstBucket := &uintHashMapBucket{}

	for i := 0; i < initSize; i++ {
		m.dir[i] = firstBucket
	}
}

func (m *UintHashMap) Clear() {
	m.init(defaultHashDirBits)
}

// find value for key
func (m *UintHashMap) Iterator() UintHashMapIterator {
	return UintHashMapIterator{m: m}
}
func (m *UintHashMap) IncludesKey(key uint) bool {
	return m.find(key, false) != nil
}
func (m *UintHashMap) Get(key uint) uint {
	e := m.find(key, false)
	if e == nil {
		return 0
	}
	return e.value
}
func (m *UintHashMap) Put(key, value uint) {
	m.find(key, true).value = value
}
func (m *UintHashMap) Inc(key, delta uint) {
	m.find(key, true).value += delta
}
func (m *UintHashMap) Dec(key, delta uint) {
	m.find(key, true).value -= delta
}
func (m *UintHashMap) Exists(key uint) bool {
	return m.find(key, false) != nil
}
func (m *UintHashMap) Delete(key uint) bool {
	if key == 0 {
		if m.zeroEntryAssigned {
			m.zeroEntryAssigned = false
			m.count--
			return true
		}
		return false
	}
	h := uintHashCode(key)
	dirIndex := h >> (bitsPerHashCode - m.dirBits)
	elemIndex := h % entriesPerHashBucket
	home := elemIndex
	b := m.dir[dirIndex]
	for {
		if b.entries[elemIndex].key == key {
			break
		}
		if b.entries[elemIndex].key == 0 {
			return false
		}

		elemIndex = (elemIndex + 1) % entriesPerHashBucket
		if elemIndex == home {
			return false
		}
	}
	b.entries[elemIndex].key = 0
	lastIndex := elemIndex
	elemIndex = (elemIndex + 1) % entriesPerHashBucket
	for elemIndex != lastIndex && b.entries[elemIndex].key != 0 {
		home = uintHashCode(b.entries[elemIndex].key) % entriesPerHashBucket
		if (lastIndex < elemIndex && (home <= lastIndex || home > elemIndex)) || (lastIndex > elemIndex && home <= lastIndex && home > elemIndex) {
			b.entries[lastIndex] = b.entries[elemIndex]
			b.entries[elemIndex].key = 0
			lastIndex = elemIndex
		}
		elemIndex = (elemIndex + 1) % entriesPerHashBucket
	}
	return true
}
func (m *UintHashMap) Len() uint {
	return m.count
}

func (m *UintHashMap) split(key uint) {
	h := uintHashCode(key)

	for {
		dirIndex := h >> (bitsPerHashCode - m.dirBits)
		splitBucket := m.dir[dirIndex]
		if splitBucket.count < entriesPerHashBucket {
			return // successfully splitted
		}
		newBits := splitBucket.bits + 1

		var workBuckets [2]*uintHashMapBucket
		workBuckets[0] = &uintHashMapBucket{bits: newBits}
		workBuckets[1] = &uintHashMapBucket{bits: newBits}

		if m.dirBits == splitBucket.bits {
			// grow directory
			newDirSize := len(m.dir) * 2
			newDir := make([]*uintHashMapBucket, newDirSize)
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
func (m *UintHashMap) find(key uint, addIfNotExists bool) *uintHashMapEntry {
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
