package hash

type uintSetBucket struct {
	bits    uint
	count   uint
	entries [entriesPerHashBucket]uint
}

type UintSetIterator struct {
	s                               *UintHashSet
	started                         bool
	curBucketIndex, curElementIndex int
}

func (it *UintSetIterator) Reset() {
	it.started = false
}

func (it *UintSetIterator) Next() bool {
	if !it.started {
		it.started = true
		it.curBucketIndex = 0
		it.curElementIndex = -1
		if it.s.hasZero {
			return true
		}
	}
	if it.curBucketIndex == len(it.s.dir) {
		return false
	}
	for {
		it.curElementIndex++
		if it.curElementIndex == entriesPerHashBucket {
			it.curElementIndex = 0
			for {
				it.curBucketIndex++
				if it.curBucketIndex == len(it.s.dir) {
					return false
				}
				if it.s.dir[it.curBucketIndex] != it.s.dir[it.curBucketIndex-1] {
					break
				}
			}
		}
		if it.s.dir[it.curBucketIndex].entries[it.curElementIndex] != 0 {
			return true
		}
	}
}
func (it *UintSetIterator) Cur() uint {
	if !it.started {
		panic("accessing unstarted iter")
	}
	if it.curElementIndex == -1 {
		return 0
	}
	return it.s.dir[it.curBucketIndex].entries[it.curElementIndex]
}

//
// UintHashSet
//
type UintHashSet struct {
	dirBits uint
	hasZero bool
	dir     []*uintSetBucket
	count   uint
}

func NewUintHashSet(initDirBits uint) *UintHashSet {
	s := &UintHashSet{}
	if initDirBits < 4 {
		initDirBits = 4
	}
	s.init(initDirBits)

	return s
}

func (s *UintHashSet) init(bits uint) {
	initSize := 1 << bits
	s.dirBits = bits
	s.dir = make([]*uintSetBucket, initSize)
	s.count = 0
	s.hasZero = false

	firstBucket := &uintSetBucket{}

	for i := 0; i < initSize; i++ {
		s.dir[i] = firstBucket
	}
}

func (s *UintHashSet) Clear() {
	s.init(4)
}

func (s *UintHashSet) Iterator() UintSetIterator {
	return UintSetIterator{s: s}
}
func (s *UintHashSet) Includes(value uint) bool {
	if value == 0 {
		return s.hasZero
	}
	return s.find(value, false)
}
func (s *UintHashSet) Add(value uint) {
	if value == 0 {
		if !s.hasZero {
			s.hasZero = true
			s.count++
		}
	} else {
		s.find(value, true)
	}
}
func (s *UintHashSet) Delete(value uint) bool {
	if value == 0 {
		if s.hasZero {
			s.hasZero = false
			s.count--
			return true
		}
		return false
	} else {
		panic("implement Delete")
	}
}
func (s *UintHashSet) Len() uint {
	return s.count
}

func (s *UintHashSet) split(value uint) {
	for {
		dirIndex := uintHashCode(value) >> (bitsPerHashCode - s.dirBits)
		splitBucket := s.dir[dirIndex]
		if splitBucket.count < entriesPerHashBucket {
			return // successfully splitted
		}
		newBits := splitBucket.bits + 1

		var workBuckets [2]*uintSetBucket
		workBuckets[0] = &uintSetBucket{bits: newBits}
		workBuckets[1] = &uintSetBucket{bits: newBits}

		if s.dirBits == splitBucket.bits {
			// grow directory
			newDirSize := len(s.dir) * 2
			newDir := make([]*uintSetBucket, newDirSize)
			for index, b := range s.dir {
				newDir[2*index] = b
				newDir[2*index+1] = b
			}
			s.dirBits = newBits
			s.dir = newDir
			dirIndex *= 2
		}

		/* Copy all elements from split bucket into the new buckets. */
		for index := 0; index < entriesPerHashBucket; index++ {
			hash := uintHashCode(splitBucket.entries[index])
			sel := (hash >> (bitsPerHashCode - newBits)) & 1
			bp := workBuckets[sel]
			elemLoc := hash % entriesPerHashBucket
			for ; bp.entries[elemLoc] != 0; elemLoc = (elemLoc + 1) % entriesPerHashBucket {
			}
			bp.entries[elemLoc] = splitBucket.entries[index]
			bp.count++
		}

		// replace splitBucket with first work bucket
		splitBucket.bits = workBuckets[0].bits
		splitBucket.count = workBuckets[0].count
		splitBucket.entries = workBuckets[0].entries

		// update dict with second bucket
		dirStart := (dirIndex >> (s.dirBits - newBits)) | 1
		dirEnd := (dirStart + 1) << (s.dirBits - newBits)
		dirStart = dirStart << (s.dirBits - newBits)

		for index := dirStart; index < dirEnd; index++ {
			s.dir[index] = workBuckets[1]
		}
	}
}

// add entry for key (or reuse existing)
func (s *UintHashSet) find(value uint, addIfNotExists bool) bool {
	valueHash := uintHashCode(value)
	dirIndex := valueHash >> (bitsPerHashCode - s.dirBits)
	elementIndex := valueHash % entriesPerHashBucket
	b := s.dir[dirIndex]
	homeIndex := elementIndex
	for {
		if b.entries[elementIndex] == value {
			return true
		}
		if b.entries[elementIndex] == 0 {
			break
		}
		elementIndex = (elementIndex + 1) % entriesPerHashBucket
		if elementIndex == homeIndex {
			break
		}
	}
	// element not found
	if !addIfNotExists {
		return false
	}
	if b.count == entriesPerHashBucket {
		s.split(value)
		dirIndex = valueHash >> (bitsPerHashCode - s.dirBits)
		b = s.dir[dirIndex]
		elementIndex = valueHash % entriesPerHashBucket
		for ; b.entries[elementIndex] != 0; elementIndex = (elementIndex + 1) % entriesPerHashBucket {
		}
	}
	b.count++
	b.entries[elementIndex] = value
	s.count++
	return true
}
