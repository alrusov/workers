package workers

import (
	"runtime"
	"sync"

	"github.com/alrusov/misc"
	"github.com/alrusov/panic"
)

//----------------------------------------------------------------------------------------------------------------------------//

// Processor --
type Processor interface {
	MaxWorkersCount() int
	ElementsCount() int
	GetElement(idx int) interface{}
	ProcInitFunc(workerID int)
	ProcFinishFunc(workerID int)
	ProcFunc(idx int, data interface{}) (err error)
}

type element struct {
	idx  int
	data interface{}
}

//----------------------------------------------------------------------------------------------------------------------------//

// Do --
func Do(p Processor) (err error) {
	elementsCount := p.ElementsCount()
	if elementsCount <= 0 {
		return
	}

	maxWorkersCount := p.MaxWorkersCount()
	if maxWorkersCount <= 0 {
		maxWorkersCount = runtime.GOMAXPROCS(-1) * 4
	}

	workersCount := maxWorkersCount
	if workersCount > elementsCount {
		workersCount = elementsCount
	}

	var wg sync.WaitGroup
	wg.Add(int(workersCount))

	msgs := misc.Messages{}

	queue := make(chan *element, elementsCount+1)
	finished := make(chan bool)

	go func() {
		defer panic.SaveStackToLog()

		for wi := 0; wi < workersCount; wi++ {
			wi := wi
			go func() {
				defer panic.SaveStackToLog()
				defer func() {
					p.ProcFinishFunc(wi)
					wg.Done()
				}()

				p.ProcInitFunc(wi)

				runtime.Gosched()

				for {
					data, more := <-queue

					// Канал закрыт
					if !more {
						return
					}

					// Получено stop-value
					if data == nil {
						finished <- true
						return
					}

					err := p.ProcFunc(data.idx, data.data)
					if err != nil {
						msgs.AddError(err)
					}
				}
			}()
		}
	}()

	for i := 0; i < elementsCount; i++ {
		queue <- &element{
			idx:  i,
			data: p.GetElement(i),
		}
	}

	// stop-value
	queue <- nil

	<-finished
	close(queue)
	wg.Wait()

	err = msgs.Error()
	if err != nil {
		return
	}

	return
}

//----------------------------------------------------------------------------------------------------------------------------//
