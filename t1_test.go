package workers

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/alrusov/misc"
)

//----------------------------------------------------------------------------------------------------------------------------//

func TestComplexZero(t *testing.T) {
	testComplex(t, 0)
}

func TestComplexOne(t *testing.T) {
	testComplex(t, 1)
}

func TestComplexSmall(t *testing.T) {
	testComplex(t, 10)
}

func TestComplexBig(t *testing.T) {
	testComplex(t, 1000000)
}

func testComplex(t *testing.T, n int) {
	w, err := New(nil)
	if err == nil {
		t.Fatalf("New(nil): Expected error but not found")
	}

	if w != nil {
		t.Fatalf("New(nil): not null worker returned")
	}

	//-------------------------------//

	check := func(fn string, maxWorker int, multithreadedThreshold int) {
		tw := &testWorker{
			data:      make([]int, n),
			clean:     false,
			withDelay: false,
		}
		for i := 0; i < n; i++ {
			tw.data[i] = i
		}

		w, err := New(
			tw,
			MaxWorker(maxWorker),
			MultithreadedThreshold(multithreadedThreshold),
		)
		if err != nil {
			t.Fatalf("New(...): %s", err.Error())
		}

		err = w.Do()
		if err != nil {
			t.Fatalf("%s: %s", fn, err.Error())
		}

		for i := 0; i < n; i++ {
			if tw.data[i] != -i {
				t.Fatalf("%s: found %d, expected %d", fn, tw.data[i], -i)
			}
		}

		st := int32(0)
		threads := maxWorker

		if maxWorker == 1 || multithreadedThreshold > n {
			st = 1
			threads = 1
		}

		if n == 0 {
			st = 0
		} else if n == 1 {
			st = 1
		}

		if threads > n {
			threads = n
		}

		if tw.singleThread != st {
			t.Fatalf("%s: single threads count is %d, expected %d", fn, tw.singleThread, st)
		}

		if tw.started != int32(threads) {
			t.Fatalf("%s: started %d threads, expected %d", fn, tw.started, threads)
		}

		if tw.finished != int32(threads) {
			t.Fatalf("%s: finished %d threads, expected %d", fn, tw.finished, threads)
		}
	}

	//-------------------------------//

	check("Singlethreaded 1", 1, 0)
	check("Singlethreaded 2", 1, n)
	check("Singlethreaded 3", 1, n+1)

	check("Multithreaded 1.1", 5, 0)
	check("Multithreaded 1.2", 5, n)
	check("Multithreaded 1.3", 5, n+1)

	check("Multithreaded 2.1", 100, 0)
	check("Multithreaded 2.2", 100, n)
	check("Multithreaded 2.3", 100, n+1)

	if n < 500 {
		check("Multithreaded 3.1", n, 0)
		check("Multithreaded 3.2", n, n)
		check("Multithreaded 3.3", n, n+1)
	}
}

//----------------------------------------------------------------------------------------------------------------------------//

func BenchmarkComplexSingleClean(b *testing.B) {
	benchmarkComplex(b, b.N, true, true)
}

func BenchmarkComplexMultiClean(b *testing.B) {
	benchmarkComplex(b, b.N, true, false)
}

func BenchmarkComplexSingle(b *testing.B) {
	benchmarkComplex(b, b.N, false, true)
}

func BenchmarkComplexMulti(b *testing.B) {
	benchmarkComplex(b, b.N, false, false)
}

func benchmarkComplex(b *testing.B, n int, clean bool, single bool) {
	tw := &testWorker{
		data:      make([]int, n),
		clean:     clean,
		withDelay: true,
	}
	for i := 0; i < n; i++ {
		tw.data[i] = i
	}

	multithreadedThreshold := 0
	if single {
		multithreadedThreshold = n + 1
	}

	w, err := New(
		tw,
		Flags(FlagFailOnError),
		MultithreadedThreshold(multithreadedThreshold),
	)
	if err != nil {
		b.Fatalf("New: %s", err.Error())
	}

	b.ResetTimer()
	err = w.Do()
	b.StopTimer()

	if err != nil {
		b.Fatalf("Do: %s", err.Error())
	}
}

//----------------------------------------------------------------------------------------------------------------------------//

type testWorker struct {
	data         []int
	singleThread int32
	started      int32
	finished     int32
	counter      int32
	clean        bool
	withDelay    bool
}

func (w *testWorker) ElementsCount() int {
	return len(w.data)
}

func (w *testWorker) GetElement(idx int) interface{} {
	return w.data[idx]
}

func (w *testWorker) ProcInitFunc(workerID int) {
	if workerID < 0 {
		atomic.AddInt32(&w.singleThread, 1)
	}
	atomic.AddInt32(&w.started, 1)
}

func (w *testWorker) ProcFinishFunc(workerID int) {
	atomic.AddInt32(&w.finished, 1)
}

func (w *testWorker) ProcFunc(idx int, data interface{}) (err error) {
	if !w.clean {
		w.data[idx] = -w.data[idx]

		atomic.AddInt32(&w.counter, 1)
		if w.withDelay {
			misc.Sleep(100 * time.Microsecond)
		}
	}
	return
}

//----------------------------------------------------------------------------------------------------------------------------//
