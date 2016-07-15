package gut

import (
	"errors"
	"fmt"
	"strconv"

	"gopkg.in/pi/goal/md"
)

var (
	OverflowError      = errors.New("overflow")
	NotPositiveError   = errors.New("expected positive number")
	TypeError          = errors.New("invalid type")
	InexactResultError = errors.New("inexact result")
)

func ToUint(arg interface{}) (uint, error) {
	switch v := arg.(type) {
	case uint:
		return v, nil
	case int:
		if v < 0 {
			return uint(v), NotPositiveError
		}
		return uint(v), nil
	case uint8:
		return uint(v), nil
	case int8:
		if v < 0 {
			return uint(v), NotPositiveError
		}
		return uint(v), nil
	case uint16:
		return uint(v), nil
	case int16:
		if v < 0 {
			return uint(v), NotPositiveError
		}
		return uint(v), nil
	case uint32:
		return uint(v), nil
	case int32:
		if v < 0 {
			return uint(v), NotPositiveError
		}
		return uint(v), nil
	case uint64:
		return uint(v), nil
	case int64:
		if v < 0 {
			return uint(v), NotPositiveError
		}
		return uint(v), nil
	case float32:
		if v < 0 {
			return 0, NotPositiveError
		}
		if (v * 1.0) != float32(uint(v)) {
			return 0, InexactResultError
		}
		return uint(v), nil
	case float64:
		if v < 0 {
			return 0, NotPositiveError
		}
		if (v * 1.0) != float64(uint(v)) {
			return 0, InexactResultError
		}
		return uint(v), nil
	default:
		return 0, TypeError
	}
}

func CheckUint(arg interface{}, term string) uint {
	if v, err := ToUint(arg); err != nil {
		panic(err.Error() + " for " + term)
	} else {
		return v
	}
}

func OptUint(arg interface{}, def uint) uint {
	if v, err := ToUint(arg); err != nil {
		return def
	} else {
		return v
	}
}

func OptUintArg(args []interface{}, index int, def uint) uint {
	if index < len(args) {
		return OptUint(args[index], def)
	} else {
		return def
	}
}

func ToInt(arg interface{}) (int, error) {
	switch v := arg.(type) {
	case uint:
		if v > uint(md.MaxInt) {
			return int(v), OverflowError
		}
		return int(v), nil
	case int:
		return v, nil
	case uint8:
		return int(v), nil
	case int8:
		return int(v), nil
	case uint16:
		return int(v), nil
	case int16:
		return int(v), nil
	case uint32:
		return int(v), nil
	case int32:
		return int(v), nil
	case uint64:
		if v > uint64(md.MaxInt) {
			return int(v), OverflowError
		} else {
			return int(v), nil
		}
	case int64:
		return int(v), nil
	case float32:
		if (v * 1.0) != float32(int(v)) {
			return 0, InexactResultError
		}
		return int(v), nil
	case float64:
		if (v * 1.0) != float64(int(v)) {
			return 0, InexactResultError
		}
		return int(v), nil
	default:
		return 0, TypeError
	}
}

func CheckInt(arg interface{}, term string) int {
	if v, err := ToInt(arg); err != nil {
		panic(err.Error() + " for " + term)
	} else {
		return v
	}
}

func OptInt(arg interface{}, def int) int {
	if v, err := ToInt(arg); err != nil {
		return def
	} else {
		return v
	}
}

func OptIntArg(args []interface{}, index int, def int) int {
	if index < len(args) {
		return OptInt(args[index], def)
	}
	return def
}

func ToFloat(arg interface{}) (float64, error) {
	switch v := arg.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case uint:
		if v > uint(md.MaxExactFloatInt) {
			return 0, OverflowError
		}
		return float64(v), nil
	case int:
		if v < md.MinExactFloatInt || v > md.MaxExactFloatInt {
			return 0, OverflowError
		}
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case uint64:
		if v > uint64(md.MaxExactFloatInt) {
			return 0, OverflowError
		}
		return float64(v), nil
	case int64:
		if v < md.MinExactFloatInt || v > md.MaxExactFloatInt {
			return 0, OverflowError
		}
		return float64(v), nil
	default:
		return 0, TypeError
	}
}

func CheckFloat(arg interface{}, term string) float64 {
	if v, err := ToFloat(arg); err != nil {
		panic(err.Error() + " for " + term)
	} else {
		return v
	}
}

func OptFloat(arg interface{}, def float64) float64 {
	if v, err := ToFloat(arg); err != nil {
		return def
	} else {
		return v
	}
}

func OptFloatArg(args []interface{}, index int, def float64) float64 {
	if index < len(args) {
		return OptFloat(args[index], def)
	}
	return def
}

func ToStr(arg interface{}) (s string) {
	switch v := arg.(type) {
	case bool:
		s = strconv.FormatBool(v)
	case float32:
		s = strconv.FormatFloat(float64(v), 'f', -1, 32)
	case float64:
		s = strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		s = strconv.FormatInt(int64(v), 10)
	case int8:
		s = strconv.FormatInt(int64(v), 10)
	case int16:
		s = strconv.FormatInt(int64(v), 10)
	case int32:
		s = strconv.FormatInt(int64(v), 10)
	case int64:
		s = strconv.FormatInt(v, 10)
	case uint:
		s = strconv.FormatUint(uint64(v), 10)
	case uint8:
		s = strconv.FormatUint(uint64(v), 10)
	case uint16:
		s = strconv.FormatUint(uint64(v), 10)
	case uint32:
		s = strconv.FormatUint(uint64(v), 10)
	case uint64:
		s = strconv.FormatUint(v, 10)
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		s = fmt.Sprintf("%v", v)
	}
	return s
}

func OptStrArg(args []interface{}, index int, def string) string {
	if index < len(args) {
		return ToStr(args[index])
	}
	return def
}
