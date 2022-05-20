package whiteflag

import (
	"encoding"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	iotago "github.com/iotaledger/iota.go/v3"
)

type Hasheable interface {
	Hash(hasher *Hasher) []byte
}

type leafValue struct {
	Value []byte
}

type hashValue struct {
	Value []byte
}

type InclusionProof struct {
	Left  Hasheable
	Right Hasheable
}

// ComputeInclusionProof computes the audit path
func (t *Hasher) ComputeInclusionProof(data []encoding.BinaryMarshaler, index int) (*InclusionProof, error) {
	if len(data) < 2 {
		return nil, errors.New("you need at lest 2 items to create an inclusion proof")
	}
	if index >= len(data) {
		return nil, fmt.Errorf("index %d out of bounds len=%d", index, len(data))
	}
	p, err := t.computeProof(data, index)
	if err != nil {
		return nil, err
	}
	return p.(*InclusionProof), nil
}

func (t *Hasher) computeProof(data []encoding.BinaryMarshaler, index int) (Hasheable, error) {
	if len(data) < 2 {
		h, err := t.Hash(data)
		if err != nil {
			return nil, err
		}
		return &hashValue{h}, nil
	}

	if len(data) == 2 {
		left, err := data[0].MarshalBinary()
		if err != nil {
			return nil, err
		}
		right, err := data[1].MarshalBinary()
		if err != nil {
			return nil, err
		}
		if index == 0 {
			return &InclusionProof{
				Left:  &leafValue{left},
				Right: &hashValue{t.hashLeaf(right)},
			}, nil
		} else {
			return &InclusionProof{
				Left:  &hashValue{t.hashLeaf(left)},
				Right: &leafValue{right},
			}, nil
		}
	}

	k := largestPowerOfTwo(len(data))
	if index < k {
		// Inside left half
		left, err := t.computeProof(data[:k], index)
		if err != nil {
			return nil, err
		}
		right, err := t.Hash(data[k:])
		if err != nil {
			return nil, err
		}
		return &InclusionProof{
			Left:  left,
			Right: &hashValue{right},
		}, nil
	} else {
		// Inside right half
		left, err := t.Hash(data[:k])
		if err != nil {
			return nil, err
		}
		right, err := t.computeProof(data[k:], index-k)
		if err != nil {
			return nil, err
		}
		return &InclusionProof{
			Left:  &hashValue{left},
			Right: right,
		}, nil
	}
}

func (l *leafValue) Hash(hasher *Hasher) []byte {
	return hasher.hashLeaf(l.Value)
}

type jsonValue struct {
	Value string `json:"value"`
}

func (l *leafValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(&jsonValue{
		Value: iotago.EncodeHex(l.Value),
	})
}

func (l *leafValue) UnmarshalJSON(bytes []byte) error {
	j := &jsonValue{}
	if err := json.Unmarshal(bytes, j); err != nil {
		return err
	}
	if len(j.Value) == 0 {
		return errors.New("missing value")
	}
	value, err := iotago.DecodeHex(j.Value)
	if err != nil {
		return err
	}
	l.Value = value
	return nil
}

func (h *hashValue) Hash(_ *Hasher) []byte {
	return h.Value
}

type jsonHash struct {
	Hash string `json:"h"`
}

func (h *hashValue) MarshalJSON() ([]byte, error) {
	return json.Marshal(&jsonHash{
		Hash: iotago.EncodeHex(h.Value),
	})
}

func (h *hashValue) UnmarshalJSON(bytes []byte) error {
	j := &jsonHash{}
	if err := json.Unmarshal(bytes, j); err != nil {
		return err
	}
	if len(j.Hash) == 0 {
		return errors.New("missing hash")
	}
	value, err := iotago.DecodeHex(j.Hash)
	if err != nil {
		return err
	}
	h.Value = value
	return nil
}

func (p *InclusionProof) Hash(hasher *Hasher) []byte {
	return hasher.hashNode(p.Left.Hash(hasher), p.Right.Hash(hasher))
}

type jsonPath struct {
	Left  *json.RawMessage `json:"l"`
	Right *json.RawMessage `json:"r"`
}

func (p *InclusionProof) MarshalJSON() ([]byte, error) {
	jsonLeft, err := json.Marshal(p.Left)
	if err != nil {
		return nil, err
	}
	jsonRight, err := json.Marshal(p.Right)
	if err != nil {
		return nil, err
	}
	rawLeft := json.RawMessage(jsonLeft)
	rawRight := json.RawMessage(jsonRight)
	return json.Marshal(&jsonPath{
		Left:  &rawLeft,
		Right: &rawRight,
	})
}

func unmarshalHashable(raw *json.RawMessage, hasheable *Hasheable) error {
	h := &hashValue{}
	if err := json.Unmarshal(*raw, h); err == nil {
		*hasheable = h
		return nil
	}
	l := &leafValue{}
	if err := json.Unmarshal(*raw, l); err == nil {
		*hasheable = l
		return nil
	}

	p := &InclusionProof{}
	err := json.Unmarshal(*raw, p)
	if err != nil {
		return err
	}
	*hasheable = p
	return nil
}

func (p *InclusionProof) UnmarshalJSON(bytes []byte) error {
	j := &jsonPath{}
	if err := json.Unmarshal(bytes, j); err != nil {
		return err
	}
	var left Hasheable
	if err := unmarshalHashable(j.Left, &left); err != nil {
		return err
	}
	var right Hasheable
	if err := unmarshalHashable(j.Right, &right); err != nil {
		return err
	}
	p.Left = left
	p.Right = right
	return nil
}
