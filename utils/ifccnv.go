package utils

import (
	"errors"

	"github.com/ardente/goal/md"
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
