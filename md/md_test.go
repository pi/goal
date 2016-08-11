package md

import (
	"testing"
	"time"
)

func TestMonotime(t *testing.T) {
	mt := Monotime()
	time.Sleep(time.Millisecond * 100)
	mt = Monotime() - mt
	if mt < time.Millisecond*95 || mt > time.Millisecond*105 {
		t.Fail()
	}
}
