package hash

//
// UintSet
// Dense set of uints. Implemented using extensive linear hashing algorithm.
//

// prefix: us

//
// UintSet contains unsigned integers without repetitions.
//
// CAUTION: never modify set during iteration!
//
type UintSet struct {
	dirBits uint        // current tree level
	hasZero bool        // special flag because 0 marks empty slot
	dir     []*usBucket // flatten tree of buckets
	count   uint        // number of elements in set (for speed up access to count)
	w       bool        // write flag, used with race detector
}

type usBucket struct {
	bits   uint
	count  uint
	values [entriesPerHashBucket]uint
}

func (s *UintSet) readaccess() {
	if s.w {
		panic("concurrent read/write")
	}
}

// inline iteration:
// for bi, ei := set.seekFirst(); bi != i1; bi, ei = set.seekNext(bi, ei) {
//		var value uint
//		if ei == -1 {
//			value = 0
//		else {
//     		value := set.dir[bi].values[ei]
//		}
//		...
// }
func (s *UintSet) seekFirst() (int, int) {
	if race {
		s.readaccess()
	}
	if s.hasZero {
		return 0, -1
	} else if s.count == 0 {
		return -1, -1
	} else {
		return s.seekNext(0, -1)
	}
}
func (s *UintSet) seekNext(bi, ei int) (int, int) {
	if race {
		s.readaccess()
	}
	if ei == -1 {
		if bi != 0 {
			panic("bad iteration order")
		}
	}
	ei++
	for {
		b := s.dir[bi]
		for ; ei < entriesPerHashBucket; ei++ {
			if b.values[ei] != 0 {
				return bi, ei
			}
		}
		for ; bi < len(s.dir) && s.dir[bi] == b; bi++ {
		}
		if bi == len(s.dir) {
			return -1, -1
		}
	}
}
func (s *UintSet) seekTo(elt uint) (int, int) {
	if race {
		s.readaccess()
	}
	if elt == 0 {
		if s.hasZero {
			return 0, -1
		} else {
			return -1, -1
		}
	}
	// locate prev element's slot
	valueHash := uintHashCode(elt)
	bi := int(valueHash >> (bitsPerHashCode - s.dirBits))
	ei := int(valueHash % entriesPerHashBucket)
	b := s.dir[bi]
	homeIndex := ei
	for {
		if b.values[ei] == elt {
			return bi, ei
		}
		if b.values[ei] == 0 {
			return -1, -1
		}
		ei = (ei + 1) % entriesPerHashBucket
		if ei == homeIndex {
			return -1, -1
		}
	}
}

//
// UintSetIterator allows iteration over set.
// CAUTION: never modify set during iteration!
//
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
		it.curBucketIndex, it.curElementIndex = it.s.seekFirst()
	} else {
		it.curBucketIndex, it.curElementIndex = it.s.seekNext(it.curBucketIndex, it.curElementIndex)
	}
	return it.curBucketIndex != -1
}
func (it *UintSetIterator) Cur() uint {
	if !it.started {
		panic("no current element")
	}
	if it.curElementIndex == -1 {
		return 0
	}
	return it.s.dir[it.curBucketIndex].values[it.curElementIndex]
}
func (it *UintSetIterator) Seek(target uint) bool {
	it.curBucketIndex, it.curElementIndex = it.s.seekTo(target)
	return it.curBucketIndex != -1
}

// UintSet methods

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

// Clone returns exact copy of the receiver
func (s *UintSet) Clone() *UintSet {
	if race {
		s.readaccess()
	}
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

// Copy returns a set with all of the receiver's elements.
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
func concwrite() {
	panic("concurrent write")
}
func (s *UintSet) beginWrite() {
	if s.w {
		concwrite()
	}
	s.w = true
}
func (s *UintSet) endWrite() {
	if s.w {
		s.w = false
	} else {
		concwrite()
	}
}
func (s *UintSet) Add(value uint) {
	if race {
		s.beginWrite()
		defer s.endWrite()
	}
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
	if race {
		s.beginWrite()
		defer s.endWrite()
	}
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
	h := uintHashCode(value)
	for {
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
			v := splitBucket.values[index]
			hash := uintHashCode(v)
			sel := (hash >> (bitsPerHashCode - newBits)) & 1
			bp := workBuckets[sel]
			elemLoc := hash % entriesPerHashBucket
			for ; bp.values[elemLoc] != 0; elemLoc = (elemLoc + 1) % entriesPerHashBucket {
			}
			bp.values[elemLoc] = v
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

// First returns first element in set with no particular order. Second return value is an indicator of element presence.
// CAUTION: never modify set during iteration!
func (s *UintSet) First() (uint, bool) {
	bi, ei := s.seekFirst()
	if bi == -1 {
		return 0, false
	}
	if ei == -1 {
		return 0, true
	}
	return s.dir[bi].values[ei], true
}

// Next returns element after prev in no particular order. Second return value is an indicator of element presence.
// CAUTION: never modify set during iteration!
func (s *UintSet) Next(prev uint) (uint, bool) {
	bi, ei := s.seekTo(prev)
	if bi == -1 {
		panic("prev not found")
	}
	bi, ei = s.seekNext(bi, ei)
	if bi == -1 {
		return 0, false
	}
	return s.dir[bi].values[ei], true
}

func (s *UintSet) Select(test func(v uint) bool) *UintSet {
	result := NewUintSet()
	for it := s.Iterator(); it.Next(); {
		cur := it.Cur()
		if test(cur) {
			result.Add(cur)
		}
	}
	return result
}

func (s *UintSet) Collect(transform func(v uint) uint) *UintSet {
	result := NewUintSet()
	for it := s.Iterator(); it.Next(); {
		result.Add(transform(it.Cur()))
	}
	return result
}

func (s *UintSet) SelectThenCollect(test func(v uint) bool, transform func(v uint) uint) *UintSet {
	result := NewUintSet()
	for it := s.Iterator(); it.Next(); {
		cur := it.Cur()
		if test(cur) {
			result.Add(transform(cur))
		}
	}
	return result
}

// Reduce appends reducer to result of prev reduction and current element
// example: calculate sum of set's elements:
// sum := set.Reduce(0, func(a, b uint) uint {return a + b})
func (s *UintSet) Reduce(initial uint, reducer func(prev, cur uint) uint) uint {
	cur := initial
	for it := s.Iterator(); it.Next(); {
		cur = reducer(cur, it.Cur())
	}
	return cur
}

// answer approx amount of used memory
func (s *UintSet) memuse() uint {
	nuq := 1
	for i := 1; i < len(s.dir); i++ {
		if s.dir[i-1] != s.dir[i] {
			nuq++
		}
	}
	return uint(nuq*entriesPerHashBucket*8 + len(s.dir)*8)
}
