package utils_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/gohornet/hornet/pkg/utils"
)

func TestMergedContextCancel(t *testing.T) {

	mergedCtx, mergedCancel := utils.MergeContexts(context.Background(), context.Background())
	mergedCancel()

	assert.True(t, func() bool {
		select {
		case <-mergedCtx.Done():
			return true
		default:
			return false
		}
	}())

	assert.Equal(t, utils.ErrMergedContextCanceled, mergedCtx.Err())
}

func TestMergedContextValues(t *testing.T) {

	ctx1, cancel1 := context.WithCancel(context.WithValue(context.Background(), "one", 1))
	defer cancel1()

	ctx2, cancel2 := context.WithCancel(context.WithValue(context.Background(), "two", 2))
	defer cancel2()

	mergedCtx, _ := utils.MergeContexts(ctx1, ctx2)

	assert.Equal(t, mergedCtx.Value("one"), 1)
	assert.Equal(t, mergedCtx.Value("two"), 2)
	assert.Nil(t, mergedCtx.Value("three"))
}

func TestMergedContextDeadline(t *testing.T) {

	deadline1 := time.Now().Add(10 * time.Second)
	deadline2 := time.Now().Add(1 * time.Second)

	ctx1, cancel1 := context.WithDeadline(context.Background(), deadline1)
	defer cancel1()

	ctx2, cancel2 := context.WithDeadline(context.Background(), deadline2)
	defer cancel2()

	mergedCtx, _ := utils.MergeContexts(ctx1, ctx2)

	deadline, ok := mergedCtx.Deadline()
	assert.False(t, deadline.IsZero())
	assert.True(t, ok)

	assert.Equal(t, deadline2, deadline)
}
