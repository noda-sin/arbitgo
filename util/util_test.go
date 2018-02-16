package util

import (
	"testing"
)

func TestFloor(t *testing.T) {
	a := 0.025342
	b := 0.01
	c := Floor(a, b)
	if c != 0.02 {
		t.Fatal("failed test")
	}
}
