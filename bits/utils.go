package bits

import "github.com/ardente/goal/utils"

func getLenAndCap(args ...interface{}) (uint, uint) {
	l := uint(0)
	c := uint(0)

	switch len(args) {
	case 0:
		return 0, 0
	case 1:
		l = utils.CheckUint(args[0], "len")
	case 2:
		l, c = utils.CheckUint(args[0], "len"), utils.CheckUint(args[1], "cap")
	default:
		panic("too many arguments")
	}
	return l, c
}
