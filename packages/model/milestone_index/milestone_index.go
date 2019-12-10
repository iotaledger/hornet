package milestone_index

type MilestoneIndex uint32

func MilestoneIndexCaller(handler interface{}, params ...interface{}) {
	handler.(func(tx MilestoneIndex))(params[0].(MilestoneIndex))
}
