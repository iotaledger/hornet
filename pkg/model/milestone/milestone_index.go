package milestone

type Index uint32

func IndexCaller(handler interface{}, params ...interface{}) {
	handler.(func(tx Index))(params[0].(Index))
}
