package hexutil

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBytesUnmarshalText(t *testing.T) {
	var tests = []*struct {
		s      string
		bytes  []byte
		expErr string
	}{
		// invalid encoding
		{s: "null", expErr: "invalid byte"},
		{s: "0", expErr: "odd length"},
		{s: "xx", expErr: "invalid byte"},
		{s: "01zz01", expErr: "invalid byte"},

		// valid encoding
		{s: "", bytes: []byte{}},
		{s: "00", bytes: []byte{0x00}},
		{s: "02", bytes: []byte{0x02}},
		{s: "ff", bytes: []byte{0xff}},
		{s: "ffffffffffffff", bytes: []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
	}

	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			var bytes Bytes
			err := bytes.UnmarshalText([]byte(tt.s))
			if tt.expErr == "" {
				assert.NoError(t, err)
			} else if assert.Error(t, err) {
				assert.Contains(t, err.Error(), tt.expErr)
			}
			assert.EqualValues(t, tt.bytes, bytes)
		})
	}
}

var bytesStringTests = []*struct {
	s string
}{
	{""},
	{"00"},
	{"02"},
	{"ff"},
	{"ffffffffffffff"},
}

func TestBytesString(t *testing.T) {
	for _, tt := range bytesStringTests {
		t.Run(tt.s, func(t *testing.T) {
			var bytes Bytes
			err := bytes.UnmarshalText([]byte(tt.s))
			require.NoError(t, err)
			assert.Equal(t, tt.s, bytes.String())
		})
	}
}

func TestBytesMarshalText(t *testing.T) {
	for _, tt := range bytesStringTests {
		t.Run(strings.ReplaceAll(tt.s, "/", "|"), func(t *testing.T) {
			var bytes Bytes
			err := bytes.UnmarshalText([]byte(tt.s))
			require.NoError(t, err)

			b, err := bytes.MarshalText()
			require.NoError(t, err)
			assert.Equal(t, []byte(tt.s), b)
		})
	}
}
