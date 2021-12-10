package milestone

import (
	"strconv"
)

type Index uint32

func (i *Index) Int() int {
	return int(*i)
}

func (i *Index) String() string {
	return strconv.Itoa(i.Int())
}

func IndexCaller(handler interface{}, params ...interface{}) {
	handler.(func(index Index))(params[0].(Index))
}
