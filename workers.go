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
		GetElement(idx int) any
		ProcInitFunc(workerID int)
		ProcFinishFunc(workerID int)
		ProcFunc(idx int, data any) (err error)
	}

	// Flags --
	Flags uint64

	// MaxWorker --
	MaxWorker uint

	// MultithreadedThreshold --
	MultithreadedThreshold uint

	element struct {
		idx  int
		data any
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
func New(processor Processor, options ...any) (w *Worker, err error) {
	if processor == nil {
		err = fmt.Errorf("processor is nil")
		return
	}

	w = &Worker{
		processor: processor,
	}

	for _, opt := range options {
		switch opt := opt.(type) {
		case Flags:
			w.flags = opt
		case MaxWorker:
			w.maxWorker = int(opt)
		case MultithreadedThreshold:
			w.multithreadedThreshold = int(opt)
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
			var data any
			if w.flags&FlagDontUseGetElement == 0 {
				data = p.GetElement(i)
			}

			err := p.ProcFunc(i, data)
			if err != nil {
				msgs.Add("[%d] %s", i, err)
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

	queue := make(chan *element, elementsCount)

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

				for active {
					data, more := <-queue

					// No more data
					if !more {
						return
					}

					err := p.ProcFunc(data.idx, data.data)
					if err != nil {
						msgs.Add("[%d] %s", data.idx, err)
						if w.flags&FlagFailOnError != 0 {
							active = false
							return
						}
					}
				}
			}()
		}
	}()

	for i := 0; i < elementsCount; i++ {
		var data any
		if w.flags&FlagDontUseGetElement == 0 {
			data = p.GetElement(i)
		}

		queue <- &element{
			idx:  i,
			data: data,
		}
	}

	close(queue)
	wg.Wait()

	err = msgs.Error()
	return
}

//----------------------------------------------------------------------------------------------------------------------------//
