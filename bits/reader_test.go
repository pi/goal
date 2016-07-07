package bits

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func ck(t *testing.T, vexp, vact uint, nexp, nact uint, err error) {
	assert.EqualValues(t, vexp, vact)
	assert.EqualValues(t, nexp, nact)
	assert.Nil(t, err)
}
func ckrz(t *testing.T, r *BitReader, nexp uint) {
	v, n, err := r.Read(1)
	ck(t, 0, v, nexp, n, err)
}
func ckeof(t *testing.T, r *BitReader) {
	v, n, err := r.Read(1)
	assert.EqualValues(t, 0, v)
	assert.EqualValues(t, 0, n)
	assert.Error(t, err)
	assert.True(t, err == io.EOF)
}

func TestBitReader(t *testing.T) {
	r := NewReader(bytes.NewReader([]byte{1, 2, 3}))
	v, n, err := r.Read(7)
	ck(t, 1, v, 7, n, err)
	ckrz(t, r, 1)
	v, n, err = r.Read(7)
	ck(t, 2, v, 7, n, err)
	ckrz(t, r, 1)
	v, n, err = r.Read(7)
	ck(t, 3, v, 7, n, err)
	ckrz(t, r, 1)
	ckeof(t, r)

	r = NewReader(bytes.NewReader([]byte{1, 2, 3, 4, 5, 6, 7, 8}))
	v, n, err = r.Read(63)
	ck(t, 0x0807060504030201, v, 63, n, err)

	v, n, err = r.Read(5)
	assert.EqualValues(t, v, 0)
	assert.EqualValues(t, 1, n)
	assert.Error(t, err)

	ckeof(t, r)
}
