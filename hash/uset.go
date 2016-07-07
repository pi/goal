package hash

//
// UintSet
// Dense set of uints. Implemented using extensive linear hashing algorithm.
//

// prefix: us

type usBucket struct {
	bits   uint
	count  uint
	values [entriesPerHashBucket]uint
}

type UintSetIterator struct {
	s                               *UintSet
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
		if it.s.dir[it.curBucketIndex].values[it.curElementIndex] != 0 {
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
	return it.s.dir[it.curBucketIndex].values[it.curElementIndex]
}

//
// UintSet
//
type UintSet struct {
	dirBits uint
	hasZero bool
	dir     []*usBucket
	count   uint
}

// Public functions

func NewUintSet() *UintSet {
	s := &UintSet{}
	s.init(4)

	return s
}

func NewUintSetWith(values []uint) *UintSet {
	s := NewUintSet()
	for _, v := range values {
		s.Add(v)
	}
	return s
}

func (s *UintSet) Clear() {
	s.init(4)
}

func (s *UintSet) Clone() *UintSet {
	r := &UintSet{}
	r.count = s.count
	r.dir = make([]*usBucket, len(s.dir))
	r.dirBits = s.dirBits
	for i, b := range s.dir {
		if i == 0 || s.dir[i] != s.dir[i-1] {
			bc := *b
			r.dir[i] = &bc
		} else {
			r.dir[i] = r.dir[i-1]
		}
	}
	return r
}

func (s *UintSet) Copy() *UintSet {
	r := NewUintSet()
	for it := r.Iterator(); it.Next(); {
		r.Add(it.Cur())
	}
	return r
}

func (s *UintSet) Iterator() UintSetIterator {
	return UintSetIterator{s: s}
}

func (s *UintSet) Includes(value uint) bool {
	if value == 0 {
		return s.hasZero
	}
	return s.find(value, false)
}

func (s *UintSet) Add(value uint) {
	if value == 0 {
		if !s.hasZero {
			s.hasZero = true
			s.count++
		}
	} else {
		s.find(value, true)
	}
}

func (s *UintSet) Delete(value uint) bool {
	if value == 0 {
		if s.hasZero {
			s.hasZero = false
			s.count--
			return true
		}
		return false
	}
	h := uintHashCode(value)
	dirIndex := h >> (bitsPerHashCode - s.dirBits)
	elemIndex := h % entriesPerHashBucket
	home := elemIndex
	b := s.dir[dirIndex]
	for {
		if b.values[elemIndex] == value {
			break
		}
		if b.values[elemIndex] == 0 {
			return false
		}

		elemIndex = (elemIndex + 1) % entriesPerHashBucket
		if elemIndex == home {
			return false
		}
	}
	b.values[elemIndex] = 0
	lastIndex := elemIndex
	elemIndex = (elemIndex + 1) % entriesPerHashBucket
	for (elemIndex != lastIndex) && (b.values[elemIndex] != 0) {
		home = uintHashCode(b.values[elemIndex]) % entriesPerHashBucket
		if (lastIndex < elemIndex && (home <= lastIndex || home > elemIndex)) || (lastIndex > elemIndex && home <= lastIndex && home > elemIndex) {
			b.values[lastIndex] = b.values[elemIndex]
			b.values[elemIndex] = 0
			lastIndex = elemIndex
		}
		elemIndex = (elemIndex + 1) % entriesPerHashBucket
	}
	s.count--
	return true
}

func (s *UintSet) Len() uint {
	return s.count
}

func (s *UintSet) Intersect(o *UintSet) *UintSet {
	r := NewUintSet()

	for it := s.Iterator(); it.Next(); {
		if o.Includes(it.Cur()) {
			r.Add(it.Cur())
		}
	}
	return r
}

func (s *UintSet) Intersects(o *UintSet) bool {
	for it := s.Iterator(); it.Next(); {
		if o.Includes(it.Cur()) {
			return true
		}
	}
	return false
}

func (s *UintSet) Union(o *UintSet) *UintSet {
	r := NewUintSet()
	for it := s.Iterator(); it.Next(); {
		r.Add(it.Cur())
	}
	for it := o.Iterator(); it.Next(); {
		r.Add(it.Cur())
	}
	return r
}

// Private functions

func (s *UintSet) init(dirBits uint) {
	initSize := 1 << dirBits
	s.dirBits = dirBits
	s.dir = make([]*usBucket, initSize)
	s.count = 0
	s.hasZero = false

	firstBucket := s.newBucket(0)

	for i := 0; i < initSize; i++ {
		s.dir[i] = firstBucket
	}
}

func (s *UintSet) newBucket(bits uint) *usBucket {
	return &usBucket{bits: bits}
}

func (s *UintSet) split(value uint) {
	for {
		h := uintHashCode(value)
		dirIndex := h >> (bitsPerHashCode - s.dirBits)
		splitBucket := s.dir[dirIndex]
		if splitBucket.count < entriesPerHashBucket {
			return // successfully splitted
		}
		newBits := splitBucket.bits + 1

		var workBuckets [2]*usBucket
		workBuckets[0] = s.newBucket(newBits)
		workBuckets[1] = s.newBucket(newBits)

		if s.dirBits == splitBucket.bits {
			// grow directory
			newDirSize := len(s.dir) * 2
			newDir := make([]*usBucket, newDirSize)
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
			hash := uintHashCode(splitBucket.values[index])
			sel := (hash >> (bitsPerHashCode - newBits)) & 1
			bp := workBuckets[sel]
			elemLoc := hash % entriesPerHashBucket
			for ; bp.values[elemLoc] != 0; elemLoc = (elemLoc + 1) % entriesPerHashBucket {
			}
			bp.values[elemLoc] = splitBucket.values[index]
			bp.count++
		}

		// replace splitBucket with first work bucket
		var di uint
		for di = h >> (bitsPerHashCode - s.dirBits); di > 0 && s.dir[di-1] == splitBucket; di-- {
		}
		for i, l := di, uint(len(s.dir)); i < l; i++ {
			if s.dir[i] != splitBucket {
				break
			}
			s.dir[i] = workBuckets[0]
		}

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
func (s *UintSet) find(value uint, addIfNotExists bool) bool {
	if s.dir == nil {
		s.init(4)
	}
	valueHash := uintHashCode(value)
	dirIndex := valueHash >> (bitsPerHashCode - s.dirBits)
	elementIndex := valueHash % entriesPerHashBucket
	b := s.dir[dirIndex]
	homeIndex := elementIndex
	for {
		if b.values[elementIndex] == value {
			return true
		}
		if b.values[elementIndex] == 0 {
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
		for ; b.values[elementIndex] != 0; elementIndex = (elementIndex + 1) % entriesPerHashBucket {
		}
	}
	b.count++
	b.values[elementIndex] = value
	s.count++
	return true
}
