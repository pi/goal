package goal

const bitsPerHashCode = _BitsPerUint

func uintHashCode(key uint) uint {
	return key * 0xc4ceb9fe1a85ec53
}
