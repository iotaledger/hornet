package tangle

import (
	"github.com/gohornet/hornet/packages/model/milestone_index"
)

func ContainsKeyHornetMilestoneIndexHornetBundle(m map[milestone_index.MilestoneIndex]*Bundle, k milestone_index.MilestoneIndex) bool {
	_, ok := m[k]
	return ok
}

func ContainsValueHornetMilestoneIndexHornetBundle(m map[milestone_index.MilestoneIndex]*Bundle, v *Bundle) bool {
	for _, mValue := range m {
		if mValue == v {
			return true
		}
	}

	return false
}

func GetKeysHornetMilestoneIndexHornetBundle(m map[milestone_index.MilestoneIndex]*Bundle) []milestone_index.MilestoneIndex {
	var keys []milestone_index.MilestoneIndex

	for k, _ := range m {
		keys = append(keys, k)
	}

	return keys
}

func GetValuesHornetMilestoneIndexHornetBundle(m map[milestone_index.MilestoneIndex]*Bundle) []*Bundle {
	var values []*Bundle

	for _, v := range m {
		values = append(values, v)
	}

	return values
}

func CopyHornetMilestoneIndexHornetBundle(m map[milestone_index.MilestoneIndex]*Bundle) map[milestone_index.MilestoneIndex]*Bundle {
	copyMap := map[milestone_index.MilestoneIndex]*Bundle{}

	for k, v := range m {
		copyMap[k] = v
	}

	return copyMap
}
