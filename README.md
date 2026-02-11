# workers


[![Go](https://img.shields.io/badge/go-1.20+-blue.svg)]()
[![License](https://img.shields.io/badge/license-MIT-green.svg)]()
[![Concurrency](https://img.shields.io/badge/Concurrency-Worker%20Pool-orange?style=flat)]()

A production-ready worker pool for parallel processing of indexed data in Go.

`workers` provides a simple and efficient framework for distributing work across goroutines with:

- configurable worker count
- graceful error handling
- panic recovery
- single-thread fallback
- zero-allocation reuse via `sync.Pool`

---

## Table of Contents

- [Features](#features)
- [Installation](#installation)
- [Concept](#concept)
- [Processor Interface](#processor-interface)
- [Usage Example](#usage-example)
- [Options](#options)
  - [Flags](#flags)
  - [MaxWorker](#maxworker)
  - [MultithreadedThreshold](#multithreadedthreshold)
- [Error Handling](#error-handling)
- [Panic Safety](#panic-safety)
- [Memory Management](#memory-management)
- [Best Practices](#best-practices)
- [License](#license)

---

## Features

- Parallel processing with configurable number of workers
- Automatic fallback to single-thread mode
- Graceful shutdown on error
- Panic recovery with stack trace logging
- Zero allocations via `sync.Pool`
- Simple `Processor` interface
- Optional `GetElement` bypass
- Error aggregation

---

## Installation

```bash
go get github.com/alrusov/workers
```

# Concept

You implement the Processor interface to define:

* how many elements exist,
* how to retrieve each element,
* and how to process them.

The worker pool distributes work across goroutines automatically.

```go
Processor Interface
type Processor interface {
    ElementsCount() int
    GetElement(idx int) any
    ProcInitFunc(workerID int)
    ProcFinishFunc(workerID int)
    ProcFunc(idx int, data any) error
}
```

# Method semantics

* ElementsCount() — returns total number of elements
* GetElement(idx) — returns element data (optional)
* ProcInitFunc(workerID) — called once per worker before processing
* ProcFinishFunc(workerID) — called once per worker after processing
* ProcFunc(idx, data) — processes one element

If workerID == -1, processing is executed in single-thread mode.

# Usage Example

```go
package main

import (
    "fmt"
    "github.com/alrusov/workers"
)

type MyProcessor struct {
    data []int
}

func (p *MyProcessor) ElementsCount() int {
    return len(p.data)
}

func (p *MyProcessor) GetElement(idx int) any {
    return p.data[idx]
}

func (p *MyProcessor) ProcInitFunc(workerID int) {
    fmt.Println("Worker started:", workerID)
}

func (p *MyProcessor) ProcFinishFunc(workerID int) {
    fmt.Println("Worker finished:", workerID)
}

func (p *MyProcessor) ProcFunc(idx int, data any) error {
    fmt.Println("Processing:", idx, data)
    return nil
}

func main() {
    p := &MyProcessor{data: []int{1, 2, 3, 4, 5}}

    w, err := workers.New(
        p,
        workers.MaxWorker(4),
    )
    if err != nil {
        panic(err)
    }
    defer w.Free()

    if err := w.Do(); err != nil {
        fmt.Println("Errors:", err)
    }
}
```

# Options

## Flags

```go
const (
    FlagDontUseGetElement = 0x0000001
    FlagFailOnError       = 0x0000002
)
```

## MaxWorker

Sets the maximum number of worker goroutines.

```
workers.MaxWorker(8)
```

If not specified, defaults to runtime.GOMAXPROCS.

## MultithreadedThreshold

Minimum element count required to enable multithreading.

```
workers.MultithreadedThreshold(100)
```

If elementsCount < threshold, processing runs in a single goroutine.

# Error Handling

All errors are collected and returned as a combined error

With FlagFailOnError enabled:

* no new tasks will start after the first error
* running tasks complete gracefully
* avoids abrupt termination

# Panic Safety

Each worker goroutine is protected with panic recovery and stack trace logging.

This ensures that panics inside worker functions do not crash the entire application.

# Memory Management

Workers are reused using sync.Pool.

Always call Free() after Do():

```
defer w.Free()
```

# Best Practices

* Always pair New() with Free()
* Prefer defer w.Free() immediately after creation
* Use FlagFailOnError for fail-fast workloads
* Avoid heavy allocations inside ProcFunc
* Keep ProcFunc stateless when possible

# License

MIT