package workerpool

import (
	"runtime"
)

var DEFAULT_OPTIONS = &Options{
	WorkerCount:          2 * runtime.NumCPU(),
	QueueSize:            4 * runtime.NumCPU(),
	FlushTasksAtShutdown: false,
}

func WorkerCount(workerCount int) Option {
	return func(args *Options) {
		args.WorkerCount = workerCount
	}
}

func QueueSize(queueSize int) Option {
	return func(args *Options) {
		args.QueueSize = queueSize
	}
}

func FlushTasksAtShutdown(flush bool) Option {
	return func(args *Options) {
		args.FlushTasksAtShutdown = flush
	}
}

type Options struct {
	WorkerCount          int
	QueueSize            int
	FlushTasksAtShutdown bool
}

func (options Options) Override(optionalOptions ...Option) *Options {
	result := &options
	for _, option := range optionalOptions {
		option(result)
	}

	return result
}

type Option func(*Options)
