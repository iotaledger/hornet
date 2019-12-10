package hornet

import (
	"github.com/iotaledger/iota.go/trinary"
	"github.com/gohornet/hornet/packages/model/milestone_index"
)

func ContainsKeyTrinaryHashMilestoneIndex(m map[trinary.Hash]milestone_index.MilestoneIndex, k trinary.Hash) bool {
	_, ok := m[k]
	return ok
}

func ContainsValueTrinaryHashMilestoneIndex(m map[trinary.Hash]milestone_index.MilestoneIndex, v milestone_index.MilestoneIndex) bool {
	for _, mValue := range m {
		if mValue == v {
			return true
		}
	}

	return false
}

func GetKeysTrinaryHashMilestoneIndex(m map[trinary.Hash]milestone_index.MilestoneIndex) []trinary.Hash {
	var keys []trinary.Hash

	for k, _ := range m {
		keys = append(keys, k)
	}

	return keys
}

func GetValuesTrinaryHashMilestoneIndex(m map[trinary.Hash]milestone_index.MilestoneIndex) []milestone_index.MilestoneIndex {
	var values []milestone_index.MilestoneIndex

	for _, v := range m {
		values = append(values, v)
	}

	return values
}

func CopyTrinaryHashMilestoneIndex(m map[trinary.Hash]milestone_index.MilestoneIndex) map[trinary.Hash]milestone_index.MilestoneIndex {
	copyMap := map[trinary.Hash]milestone_index.MilestoneIndex{}

	for k, v := range m {
		copyMap[k] = v
	}

	return copyMap
}
