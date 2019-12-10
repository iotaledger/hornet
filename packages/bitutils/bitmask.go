package bitutils

type BitMask byte

func (bitmask BitMask) SettingFlag(pos uint) BitMask {
	return bitmask | (1 << pos)
}

func (bitmask BitMask) ClearingFlag(pos uint) BitMask {
	return bitmask & ^(1 << pos)
}

func (bitmask BitMask) ModifyingFlag(pos uint, state bool) BitMask {
	if state {
		return bitmask.SettingFlag(pos)
	} else {
		return bitmask.ClearingFlag(pos)
	}
}

func (bitmask BitMask) HasFlag(pos uint) bool {
	return (bitmask&(1<<pos) > 0)
}
