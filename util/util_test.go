package util

import (
	"testing"
)

func TestFloor(t *testing.T) {
	a := 10.46405786
	b := 0.010000000
	c := Floor(a, b)
	if c != 10.46 {
		t.Fatal("failed test")
	}
}
