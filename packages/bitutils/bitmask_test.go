package bitutils

import (
	"testing"
)

func TestBitmask(t *testing.T) {
	var b BitMask

	if b.HasFlag(0) {
		t.Error("flag at pos 0 should not be set")
	}
	if b.HasFlag(1) {
		t.Error("flag at pos 1 should not be set")
	}

	b = b.SettingFlag(0)
	if !b.HasFlag(0) {
		t.Error("flag at pos 0 should be set")
	}
	b = b.SettingFlag(1)
	if !b.HasFlag(1) {
		t.Error("flag at pos 1 should be set")
	}

	b = b.ModifyingFlag(0, false)
	if b.HasFlag(0) {
		t.Error("flag at pos 0 should not be set")
	}
	b = b.ClearingFlag(1)
	if b.HasFlag(1) {
		t.Error("flag at pos 1 should not be set")
	}
}
