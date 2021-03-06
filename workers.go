package workers

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/alrusov/misc"
	"github.com/alrusov/panic"
)

//----------------------------------------------------------------------------------------------------------------------------//

type (
	// Worker --
	Worker struct {
		processor              Processor
		flags                  Flags
		maxWorker              int
		multithreadedThreshold int
	}

	// Processor --
	Processor interface {
		ElementsCount() int
		GetElement(idx int) interface{}
		ProcInitFunc(workerID int)
		ProcFinishFunc(workerID int)
		ProcFunc(idx int, data interface{}) (err error)
	}

	// Flags --
	Flags uint64

	// MaxWorker --
	MaxWorker uint

	// MultithreadedThreshold --
	MultithreadedThreshold uint

	element struct {
		idx  int
		data interface{}
	}
)

const (
	// FlagDontUseGetElement --
	FlagDontUseGetElement = 0x0000001
	// FlagFailOnError --
	FlagFailOnError = 0x0000002
)

//----------------------------------------------------------------------------------------------------------------------------//

// New --
func New(processor Processor, options ...interface{}) (w *Worker, err error) {
	if processor == nil {
		err = fmt.Errorf("processor is nil")
		return
	}

	w = &Worker{
		processor: processor,
	}

	for _, opt := range options {
		switch opt.(type) {
		case Flags:
			w.flags = opt.(Flags)
		case MaxWorker:
			w.maxWorker = int(opt.(MaxWorker))
		case MultithreadedThreshold:
			w.multithreadedThreshold = int(opt.(MultithreadedThreshold))
		}
	}

	return
}

//----------------------------------------------------------------------------------------------------------------------------//

// Do --
func (w *Worker) Do() (err error) {
	p := w.processor

	elementsCount := p.ElementsCount()
	if elementsCount <= 0 {
		return
	}

	workersCount := w.maxWorker
	if workersCount <= 0 {
		workersCount = runtime.GOMAXPROCS(-1) * 4
	}

	if workersCount > elementsCount {
		workersCount = elementsCount
	}

	multithreadedThreshold := w.multithreadedThreshold
	if multithreadedThreshold < 0 {
		multithreadedThreshold = workersCount * 50
	}

	msgs := misc.NewMessages()

	if workersCount == 1 || multithreadedThreshold > elementsCount {
		// Singlethread
		p.ProcInitFunc(-1)

		for i := 0; i < elementsCount; i++ {
			var data interface{}
			if w.flags&FlagDontUseGetElement == 0 {
				data = p.GetElement(i)
			}

			err := p.ProcFunc(i, data)
			if err != nil {
				msgs.AddError(err)
				if w.flags&FlagFailOnError != 0 {
					break
				}
			}
		}

		p.ProcFinishFunc(-1)

		err = msgs.Error()
		return
	}

	// Multithread
	var wg sync.WaitGroup
	wg.Add(int(workersCount))

	queue := make(chan element, elementsCount+1)
	finished := make(chan bool, workersCount)

	go func() {
		panicID := panic.ID()
		defer panic.SaveStackToLogEx(panicID)

		active := true

		for wi := 0; wi < workersCount; wi++ {
			wi := wi
			go func() {
				panicID := panic.ID()
				defer panic.SaveStackToLogEx(panicID)
				defer func() {
					p.ProcFinishFunc(wi)
					wg.Done()
				}()

				p.ProcInitFunc(wi)

				runtime.Gosched()

				for active {
					data, more := <-queue

					// Channel is closed
					if !more {
						return
					}

					// stop-value received
					if data.idx < 0 {
						finished <- true
						return
					}

					err := p.ProcFunc(data.idx, data.data)
					if err != nil {
						msgs.AddError(err)
						if w.flags&FlagFailOnError != 0 {
							active = false
							finished <- true
							return
						}
					}
				}
			}()
		}
	}()

	for i := 0; i < elementsCount; i++ {
		var data interface{}
		if w.flags&FlagDontUseGetElement == 0 {
			data = p.GetElement(i)
		}

		queue <- element{
			idx:  i,
			data: data,
		}
	}

	// stop-value
	queue <- element{
		idx: -1,
	}

	<-finished
	close(queue)
	wg.Wait()

	close(finished)

	err = msgs.Error()
	return
}

//----------------------------------------------------------------------------------------------------------------------------//
