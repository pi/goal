package goal

const bitsPerHashCode = 64

func uintHashCode(key uint) uint {
	return key * 0xc4ceb9fe1a85ec53
}
