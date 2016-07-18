package bits

import "github.com/pi/goal/gut"

func getLenAndCap(args ...interface{}) (uint, uint) {
	l := uint(0)
	c := uint(0)

	switch len(args) {
	case 0:
		return 0, 0
	case 1:
		l = gut.CheckUint(args[0], "len")
	case 2:
		l, c = gut.CheckUint(args[0], "len"), gut.CheckUint(args[1], "cap")
	default:
		panic("too many arguments")
	}
	return l, c
}
