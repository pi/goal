package utils

import "testing"

func TestHighBit(t *testing.T) {
	if HighBit(0) != -1 {
		t.Fail()
	}
	for i := uint(0); i < 63; i++ {
		if HighBit(uint(1<<i)) != int(i) {
			t.Fail()
		}
	}
}
