package worker

import (
	"sync"
	"testing"
)

func TestPortAllocator_AcquireRelease(t *testing.T) {
	a := NewPortAllocator(40000, 40009) // 10 ports
	got := make(map[int]bool)
	for i := 0; i < 10; i++ {
		p, err := a.Acquire()
		if err != nil {
			t.Fatalf("acquire %d: %v", i, err)
		}
		if p < 40000 || p > 40009 {
			t.Fatalf("port %d outside range", p)
		}
		if got[p] {
			t.Fatalf("port %d handed out twice", p)
		}
		got[p] = true
	}
	if _, err := a.Acquire(); err != ErrPortsExhausted {
		t.Fatalf("expected ErrPortsExhausted, got %v", err)
	}
	for p := range got {
		a.Release(p)
	}
	if _, err := a.Acquire(); err != nil {
		t.Fatalf("expected reacquire after release, got %v", err)
	}
}

func TestPortAllocator_ConcurrentAcquireUnique(t *testing.T) {
	a := NewPortAllocator(41000, 41063) // 64 ports
	const N = 64
	var wg sync.WaitGroup
	mu := sync.Mutex{}
	seen := make(map[int]bool)
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := a.Acquire()
			if err != nil {
				errs <- err
				return
			}
			mu.Lock()
			defer mu.Unlock()
			if seen[p] {
				errs <- ErrPortsExhausted // sentinel for "duplicate"
				return
			}
			seen[p] = true
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent acquire failure: %v", err)
	}
	if len(seen) != N {
		t.Fatalf("expected %d unique ports, got %d", N, len(seen))
	}
}

func TestPortAllocator_ReverseRange(t *testing.T) {
	a := NewPortAllocator(42010, 42000) // swapped
	p, err := a.Acquire()
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	if p < 42000 || p > 42010 {
		t.Fatalf("port %d outside [42000,42010]", p)
	}
}
