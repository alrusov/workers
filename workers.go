package workers

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"

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
	MaxWorker int

	// MultithreadedThreshold --
	MultithreadedThreshold int
)

const (
	FlagDontUseGetElement = 0x0000001 // Dont use GetElement function
	FlagFailOnError       = 0x0000002 // Fail on error
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
		workersCount = runtime.GOMAXPROCS(-1)
	}

	if workersCount > elementsCount {
		workersCount = elementsCount
	}

	multithreadedThreshold := w.multithreadedThreshold
	if multithreadedThreshold < 0 {
		multithreadedThreshold = workersCount * 50
	}

	msgs := misc.NewMessages()
	defer msgs.Free()

	if workersCount == 1 || multithreadedThreshold > elementsCount {
		// Single thread
		p.ProcInitFunc(-1)

		var data any

		for i := 0; i < elementsCount; i++ {
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

	currIdx := int32(-1)
	active := true

	for wi := 0; wi < workersCount; wi++ {
		wi := wi
		go func() {
			panicID := panic.ID()
			defer panic.SaveStackToLogEx(panicID)

			p.ProcInitFunc(wi)

			defer func() {
				p.ProcFinishFunc(wi)
				wg.Done()
			}()

			var data any

			for active {
				idx := int(atomic.AddInt32(&currIdx, 1))
				if idx >= elementsCount {
					// No more data
					break
				}

				if w.flags&FlagDontUseGetElement == 0 {
					data = p.GetElement(idx)
				}

				err := p.ProcFunc(idx, data)
				if err != nil {
					msgs.Add("[%d] %s", idx, err)
					if w.flags&FlagFailOnError != 0 {
						active = false
						break
					}
				}
			}
		}()
	}

	wg.Wait()
	err = msgs.Error()
	return
}

//----------------------------------------------------------------------------------------------------------------------------//
