package tunnel

import (
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
)

// async is a helper function to convert a blocking function to a function
// returning an error. Useful for plugging function closures into select and co
func async(fn func() error) <-chan error {
	errChan := make(chan error, 0)
	go func() {
		select {
		case errChan <- fn():
		default:
		}

		close(errChan)
	}()

	return errChan
}

type expBackoff struct {
	mu sync.Mutex
	bk *backoff.ExponentialBackOff
}

func newForeverBackoff() *expBackoff {
	eb := &expBackoff{
		bk: backoff.NewExponentialBackOff(),
	}
	eb.bk.MaxElapsedTime = 0 // never stops
	return eb
}

func (eb *expBackoff) NextBackOff() time.Duration {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	return eb.bk.NextBackOff()
}

func (eb *expBackoff) Reset() {
	eb.mu.Lock()
	eb.bk.Reset()
	eb.mu.Unlock()
}

type callbacks struct {
	mu    sync.Mutex
	name  string
	funcs map[string]func() error
}

func newCallbacks(name string) *callbacks {
	return &callbacks{
		name:  name,
		funcs: make(map[string]func() error),
	}
}

func (c *callbacks) add(ident string, fn func() error) {
	c.mu.Lock()
	c.funcs[ident] = fn
	c.mu.Unlock()
}

func (c *callbacks) pop(ident string) (func() error, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	fn, ok := c.funcs[ident]
	if !ok {
		return nil, nil // nop
	}

	delete(c.funcs, ident)

	if fn == nil {
		return nil, fmt.Errorf("%s: nil callback set for %q client", ident)
	}

	return fn, nil
}

func (c *callbacks) call(ident string) error {
	fn, err := c.pop(ident)
	if err != nil {
		return err
	}

	if fn == nil {
		return nil // nop
	}

	return fn()
}
