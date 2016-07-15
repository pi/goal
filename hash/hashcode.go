package hash

import "gopkg.in/pi/goal/md"

const bitsPerHashCode = md.BitsPerUint

func uintHashCode(key uint) uint {
	return key * 0xc4ceb9fe1a85ec53
}
