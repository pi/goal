package gut

import (
	"testing"

	"github.com/pi/goal/md"
	"github.com/stretchr/testify/assert"
)

func TestToUint(t *testing.T) {
	goodVals := []interface{}{
		1,
		int8(3),
		uint8(3),
		int16(3),
		uint16(3),
		int32(3),
		uint32(3),
		int64(3),
		uint64(3),
		int(3),
		uint(3),
		float32(100),
		float64(100),
	}

	for _, arg := range goodVals {
		actual, err := ToUint(arg)
		assert.NoError(t, err, "convertsion error of %v", arg)
		assert.EqualValues(t, arg, actual)
	}

	badVals := []interface{}{
		-1,
		int(-1),
		int8(-1),
		int16(-1),
		int32(-1),
		int64(-1),
		float32(-1),
		float64(-1),
		"zork",
		float32(1.5),
		float64(1.5),
	}

	for _, bv := range badVals {
		_, err := ToUint(bv)
		assert.Error(t, err, "no conversion error for %v", bv)
	}
	for _, bv := range badVals {
		assert.Panics(t, func() { CheckUint(bv, "t") })
	}

	cke := func(v interface{}, e error) {
		_, err := ToUint(v)
		assert.Equal(t, e, err)
	}
	cke("a", TypeError)
	cke(-1, NotPositiveError)
	cke(float32(3.000001), InexactResultError)
	cke(float64(3.00000001), InexactResultError)
	// test Opt/Arg
	t.Fail()
}

func TestToInt(t *testing.T) {
	goodVals := []interface{}{
		1,
		int8(3),
		uint8(3),
		int16(3),
		uint16(3),
		int32(3),
		uint32(3),
		int64(3),
		uint64(3),
		int(3),
		uint(3),
		int8(-3),
		int16(-3),
		int32(-3),
		int64(-3),
		int(-3),
		float32(-3),
		float64(-3),
	}

	for _, arg := range goodVals {
		actual, err := ToInt(arg)
		assert.NoError(t, err, "convertsion error of %v", arg)
		assert.EqualValues(t, arg, actual)
	}

	badVals := []interface{}{
		uint(1) << (md.BitsPerUint - 1),
		uint(md.MaxInt) + 1,
		"zork",
		3.3,
	}
	for _, bv := range badVals {
		_, err := ToInt(bv)
		assert.Error(t, err, "no conversion error for %v", bv)
		assert.Panics(t, func() { CheckInt(bv, "t") })
	}

	cke := func(v interface{}, e error) {
		_, err := ToInt(v)
		assert.Equal(t, e, err)
	}
	cke("a", TypeError)
	cke(uint(md.MaxInt)+1, OverflowError)
	cke(uint(1)<<63, OverflowError)
	cke(float32(-1.00001), InexactResultError)
	cke(float64(-1.00001), InexactResultError)
	//TODO test Opt/Arg
	t.Fail()
}

func TestToFloat(t *testing.T) {
	goodVals := []interface{}{
		1,
		int8(3),
		uint8(3),
		int16(3),
		uint16(3),
		int32(3),
		uint32(3),
		int64(3),
		uint64(3),
		int(3),
		uint(3),
		int8(-3),
		int16(-3),
		int32(-3),
		int64(-3),
		int(-3),
		float64(3),
		float32(3),
	}

	for _, arg := range goodVals {
		actual, err := ToFloat(arg)
		assert.NoError(t, err, "convertsion error of %v", arg)
		assert.EqualValues(t, arg, actual)
	}

	badVals := []interface{}{
		[]byte{1, 2, 3},
		"zork",
		md.MaxExactFloat64Int + 1,
		md.MinExactFloat64Int - 1,
	}

	for _, bv := range badVals {
		_, err := ToFloat(bv)
		assert.Error(t, err, "no conversion error for %v", bv)
		assert.Panics(t, func() { CheckFloat(bv, "test") })
	}

	cke := func(v interface{}, e error) {
		_, err := ToFloat(v)
		assert.Equal(t, e, err)
	}
	cke("a", TypeError)
	cke(md.MaxExactFloat64Int+3, OverflowError)
	cke(md.MinExactFloat64Int-3, OverflowError)

	//TODO test Opt/Arg
	t.Fail()
}

func TestToStr(t *testing.T) {
	//TODO
	t.Fail()
}
