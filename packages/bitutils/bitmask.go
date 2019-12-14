package bitutils

// BitMask type
type BitMask byte

// SettingFlag sets a flag at a given position
func (bitmask BitMask) SettingFlag(pos uint) BitMask {
	return bitmask | (1 << pos)
}

// ClearingFlag resets a flag at a given position
func (bitmask BitMask) ClearingFlag(pos uint) BitMask {
	return bitmask & ^(1 << pos)
}

// ModifyingFlag modifies a flag at a given position to a given state
func (bitmask BitMask) ModifyingFlag(pos uint, state bool) BitMask {
	if state {
		return bitmask.SettingFlag(pos)
	}
	return bitmask.ClearingFlag(pos)
}

// HasFlag returns the state of a flag at a given position
func (bitmask BitMask) HasFlag(pos uint) bool {
	return (bitmask&(1<<pos) > 0)
}
