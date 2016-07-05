package goal

type word uint

const _BitsPerChunkSizeBits = 13
const _BitsPerChunk = 1 << _BitsPerChunkSizeBits
const _BitsPerChunkSizeMask = _BitsPerChunk - 1
const _WordSizeBits = 6
const _BitsPerWord = 1 << _WordSizeBits
const _WordBitsMask = _BitsPerWord - 1
const _WordsPerChunk = _BitsPerChunk >> _WordSizeBits

type bitChunk [_WordsPerChunk]word

type BitArray struct {
	limit  int
	chunks map[uint]*bitChunk
}

func NewBitArray(limit int) *BitArray {
	return &BitArray{
		chunks: make(map[uint]*bitChunk),
		limit:  limit,
	}
}

func (a *BitArray) Len() int {
	return a.limit
}

func (a *BitArray) Put(index int, value bool) {
	if index < 0 || index >= a.limit {
		panic("bit array index out of bounds")
	}
	chunkIndex := uint(index >> _BitsPerChunkSizeBits)
	chunk, exists := a.chunks[chunkIndex]
	if !exists {
		chunk = new(bitChunk)
		a.chunks[chunkIndex] = chunk
	}
	wi := uint(index&_BitsPerChunkSizeMask) >> _BitsPerWord
	bi := uint(index & _WordBitsMask)
	if value {
		(*chunk)[wi] |= 1 << bi
	} else {
		(*chunk)[wi] &= ^(1 << bi)
	}
}

func (a *BitArray) Get(index int) bool {
	if index < 0 || index >= a.limit {
		panic("bit array index out of bounds")
	}
	chunkIndex := uint(index >> _BitsPerChunkSizeBits)
	chunk, exists := a.chunks[chunkIndex]
	if !exists {
		return false
	}
	return (((*chunk)[uint(index&_BitsPerChunkSizeMask)>>_BitsPerWord]) & (1 << uint(index&_WordBitsMask))) != 0
}

func (a *BitArray) Clear() {
	a.chunks = make(map[uint]*bitChunk)
}
