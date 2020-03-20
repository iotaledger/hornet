package milestone

type Index uint32

func MilestoneIndexCaller(handler interface{}, params ...interface{}) {
	handler.(func(tx Index))(params[0].(Index))
}
