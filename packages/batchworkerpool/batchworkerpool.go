package batchworkerpool

import (
	"sync"
	"time"

	"github.com/gohornet/hornet/packages/syncutils"
)

type BatchWorkerPool struct {
	workerFnc func([]Task)
	options   *Options

	calls                 chan Task
	batchedCalls          chan []Task
	terminate             chan struct{}
	terminateBatchWorkers chan struct{}

	running  bool
	shutdown bool

	mutex syncutils.RWMutex
	wait  sync.WaitGroup
}

func New(workerFnc func([]Task), optionalOptions ...Option) (result *BatchWorkerPool) {
	options := DEFAULT_OPTIONS.Override(optionalOptions...)

	result = &BatchWorkerPool{
		workerFnc:             workerFnc,
		options:               options,
		calls:                 make(chan Task, options.QueueSize),
		batchedCalls:          make(chan []Task, 2*options.WorkerCount),
		terminate:             make(chan struct{}),
		terminateBatchWorkers: make(chan struct{}),
	}

	return
}

func (wp *BatchWorkerPool) Submit(params ...interface{}) (result chan interface{}) {

	wp.mutex.RLock()

	if !wp.shutdown {
		result = make(chan interface{}, 1)

		wp.calls <- Task{
			params:     params,
			resultChan: result,
		}
	}

	wp.mutex.RUnlock()

	return
}

func (wp *BatchWorkerPool) Start() {
	wp.mutex.Lock()

	if !wp.running {
		if wp.shutdown {
			panic("BatchWorker was already used before")
		}
		wp.running = true

		wp.startBatchDispatcher()
		wp.startBatchWorkers()
	}

	wp.mutex.Unlock()
}

func (wp *BatchWorkerPool) Run() {
	wp.Start()

	wp.wait.Wait()
}

func (wp *BatchWorkerPool) Stop() {
	wp.mutex.Lock()

	if wp.running {
		wp.shutdown = true
		wp.running = false

		close(wp.terminate)
	}

	wp.mutex.Unlock()
}

func (wp *BatchWorkerPool) StopAndWait() {
	wp.Stop()
	wp.wait.Wait()
}

func (wp *BatchWorkerPool) GetWorkerCount() int {
	return wp.options.WorkerCount
}

func (wp *BatchWorkerPool) GetBatchSize() int {
	return wp.options.BatchSize
}

func (wp *BatchWorkerPool) GetPendingQueueSize() int {
	return len(wp.batchedCalls)
}

func (wp *BatchWorkerPool) dispatchTasks(task Task) {

	batchTask := append(make([]Task, 0), task)

	collectionTimeout := time.After(wp.options.BatchCollectionTimeout)

	// collect additional requests that arrive within the timeout
CollectAdditionalCalls:
	for {
		select {

		case <-collectionTimeout:
			break CollectAdditionalCalls

		case call := <-wp.calls:
			batchTask = append(batchTask, call)

			if len(batchTask) == wp.options.BatchSize {
				break CollectAdditionalCalls
			}
		}
	}

	wp.batchedCalls <- batchTask
}

func (wp *BatchWorkerPool) startBatchDispatcher() {
	wp.wait.Add(1)

	go func() {
		for {
			select {
			case <-wp.terminate:

			terminateLoop:
				// process all waiting tasks after shutdown signal
				for {
					select {
					case firstCall := <-wp.calls:
						wp.dispatchTasks(firstCall)

					default:
						break terminateLoop
					}
				}

				close(wp.terminateBatchWorkers)

				wp.wait.Done()
				return

			case firstCall := <-wp.calls:
				wp.dispatchTasks(firstCall)
			}
		}
	}()
}

func (wp *BatchWorkerPool) startBatchWorkers() {

	for i := 0; i < wp.options.WorkerCount; i++ {
		wp.wait.Add(1)

		go func() {
			aborted := false

			for !aborted {
				select {
				case <-wp.terminateBatchWorkers:
					aborted = true

				terminateLoop:
					// process all waiting tasks after shutdown signal
					for {
						select {
						case batchTask := <-wp.batchedCalls:
							wp.workerFnc(batchTask)

						default:
							break terminateLoop
						}
					}

				case batchTask := <-wp.batchedCalls:
					wp.workerFnc(batchTask)
				}
			}

			wp.wait.Done()
		}()
	}
}
