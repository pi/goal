package hash

//
// UintMap
// Dense map of uint->uint.
// Implemented using extendible hashing mechanism
//

// prefix: uum

import (
	"gopkg.in/pi/goal/md"
)

type uumEntry struct {
	key, value uint
}

type uumBucket struct {
	bits    uint
	count   uint
	entries [entriesPerHashBucket]uumEntry
}

type UintMapIterator struct {
	m               *UintMap
	started         bool
	curBucketIndex  int
	curElementIndex int
}

func (it *UintMapIterator) Reset() {
	it.started = false
}

func (it *UintMapIterator) Next() bool {
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
func (it *UintMapIterator) CurKey() uint {
	if !it.started {
		panic("accessing unstarted iterator")
	}
	if it.curElementIndex == -1 {
		return 0 // zero entry key
	}
	return it.m.dir[it.curBucketIndex].entries[it.curElementIndex].key
}

// Return current map value. Panic if the iterator has not been started.
func (it *UintMapIterator) Cur() uint {
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
type UintMap struct {
	dirBits           uint
	zeroEntry         uumEntry
	zeroEntryAssigned bool
	dir               []*uumBucket
	count             uint
}

func NewUintMap(args ...interface{}) *UintMap {
	m := &UintMap{}
	initBits := uint(4)
	if len(args) > 1 {
		panic("usage: NewUintMap([initDirBits])")
	}
	if len(args) == 1 {
		if v, ok := args[0].(uint); ok {
			initBits = v
		} else if v, ok := args[0].(int); ok {
			initBits = uint(v)
		} else if v, ok := args[0].(uint64); ok {
			initBits = uint(v)
		} else if v, ok := args[0].(int64); ok {
			initBits = uint(v)
		} else {
			panic("expected integer init size")
		}
		if initBits < 2 || initBits > (md.BitsPerUint-3) {
			panic("invalid init bits")
		}
	}

	m.init(initBits)

	return m
}

func (m *UintMap) init(bits uint) {
	initSize := 1 << bits
	m.dirBits = bits
	m.dir = make([]*uumBucket, initSize)
	m.count = 0
	m.zeroEntryAssigned = false

	firstBucket := &uumBucket{}

	for i := 0; i < initSize; i++ {
		m.dir[i] = firstBucket
	}
}

func (m *UintMap) Clear() {
	m.init(defaultHashDirBits)
}

// find value for key
func (m *UintMap) Iterator() UintMapIterator {
	return UintMapIterator{m: m}
}
func (m *UintMap) IncludesKey(key uint) bool {
	return m.find(key, false) != nil
}
func (m *UintMap) Get(key uint) uint {
	e := m.find(key, false)
	if e == nil {
		return 0
	}
	return e.value
}
func (m *UintMap) Put(key, value uint) {
	m.find(key, true).value = value
}
func (m *UintMap) Inc(key, delta uint) {
	m.find(key, true).value += delta
}
func (m *UintMap) Dec(key, delta uint) {
	m.find(key, true).value -= delta
}
func (m *UintMap) Exists(key uint) bool {
	return m.find(key, false) != nil
}
func (m *UintMap) Delete(key uint) bool {
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
func (m *UintMap) Len() uint {
	return m.count
}
func (m *UintMap) DirSize() int {
	return len(m.dir)
}
func (m *UintMap) BucketCount() int {
	c := 1
	for i := 1; i < len(m.dir); i++ {
		if m.dir[i] != m.dir[i-1] {
			c++
		}
	}
	return c
}
func (m *UintMap) Do(f func(uint, uint)) {
	if m.zeroEntryAssigned {
		f(0, m.zeroEntry.value)
	}
	di := 0
	for {
		b := m.dir[di]
		for i := 0; i < entriesPerHashBucket; i++ {
			if b.entries[i].key != 0 {
				f(b.entries[i].key, b.entries[i].value)
			}
		}
		for di++; di < len(m.dir) && m.dir[di] == m.dir[di-1]; di++ {
		}
		if di == len(m.dir) {
			return
		}
	}
}

func (m *UintMap) split(key uint) {
	h := uintHashCode(key)

	for {
		dirIndex := h >> (bitsPerHashCode - m.dirBits)
		splitBucket := m.dir[dirIndex]
		if splitBucket.count < entriesPerHashBucket {
			return // successfully splitted
		}
		newBits := splitBucket.bits + 1

		var workBuckets [2]*uumBucket
		workBuckets[0] = &uumBucket{bits: newBits}
		workBuckets[1] = &uumBucket{bits: newBits}

		if m.dirBits == splitBucket.bits {
			// grow directory
			newDirSize := len(m.dir) * 2
			newDir := make([]*uumBucket, newDirSize)
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
		var di uint
		for di = h >> (bitsPerHashCode - m.dirBits); di > 0 && m.dir[di-1] == splitBucket; di-- {
		}
		for i, l := di, uint(len(m.dir)); i < l; i++ {
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
func (m *UintMap) find(key uint, addIfNotExists bool) *uumEntry {
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
