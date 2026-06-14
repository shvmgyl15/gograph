package concurrent

import (
	"os"
	"sync"
)

// Stringer is a simple interface.
type Stringer interface {
	String() string
}

// Worker is a struct that satisfies Stringer.
type Worker struct {
	mu   sync.Mutex
	Name string
}

// String satisfies the Stringer interface.
func (w *Worker) String() string {
	return w.Name
}

// Run spawns a goroutine and uses a mutex.
func (w *Worker) Run(ch chan<- string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	go func() {
		defer func() {
			_ = recover()
		}()
		ch <- w.Name
	}()
}

// Start uses a WaitGroup.
func Start(workers []*Worker, ch chan<- string) {
	var wg sync.WaitGroup
	for _, w := range workers {
		wg.Add(1)
		go func(worker *Worker) {
			defer func() {
				_ = recover()
			}()
			defer wg.Done()
			worker.Run(ch)
		}(w)
	}
	wg.Wait()
}

// GetEnv reads configuration from the environment.
func GetEnv() string {
	return os.Getenv("WORKER_NAME")
}
