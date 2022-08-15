package protocol

var (
	SupportedVersions = Versions{2} // make sure to add the versions sorted asc
)

// Versions is a slice of protocol versions.
type Versions []uint32

// Highest returns the highest version.
func (v Versions) Highest() byte {
	return byte(v[len(v)-1])
}

// Lowest returns the lowest version.
func (v Versions) Lowest() byte {
	return byte(v[0])
}

// Supports tells whether the given version is supported.
func (v Versions) Supports(ver byte) bool {
	for _, value := range v {
		if value == uint32(ver) {
			return true
		}
	}

	return false
}
