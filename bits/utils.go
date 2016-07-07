package bits

func errNegative(term string) {
	panic("expected positive integer for " + term)
}

func toUint(arg interface{}, term string) uint {
	switch v := arg.(type) {
	case uint:
		return v
	case uint8:
		return uint(v)
	case uint64:
		return uint(v)
	case int:
		if v < 0 {
			errNegative(term)
		}
		return uint(v)
	case int8:
		if v < 0 {
			errNegative(term)
		}
		return uint(v)
	case int32:
		if v < 0 {
			errNegative(term)
		}
		return uint(v)
	case int64:
		if v < 0 {
			errNegative(term)
		}
		return uint(v)
	default:
		panic("expected positive integer for " + term)
	}
}

func getLenAndCap(args ...interface{}) (uint, uint) {
	l := uint(0)
	c := uint(0)

	switch len(args) {
	case 0:
		return 0, 0
	case 1:
		l = toUint(args[0], "len")
	case 2:
		l, c = toUint(args[0], "len"), toUint(args[1], "cap")
	default:
		panic("too many arguments")
	}
	return l, c
}
