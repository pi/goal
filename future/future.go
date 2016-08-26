package future

import (
	"time"
)

type futureFunc func(*Future) error
type errHandler func(*Future)
type completionFunc func(*Future)

type Future struct {
	funcs        []futureFunc
	errHandlers  []errHandler
	doneHandlers []completionFunc
	c            chan struct{}
	err          error
	result       interface{}
}

func New(fn futureFunc) *Future {
	f := &Future{}
	f.addFunc(fn)
	return f
}

func (f *Future) addFunc(fn futureFunc) {
	f.funcs = append(f.funcs, fn)
	f.errHandlers = append(f.errHandlers, nil)
	f.doneHandlers = append(f.doneHandlers, nil)
}

func (f *Future) runOne(index int, rc chan error) error {
	err := f.funcs[index](f)
	if rc != nil {
		rc <- err
	}
	if err == nil && f.doneHandlers[index] != nil {
		f.doneHandlers[index](f)
	}
	return err
}

func (f *Future) run() {
	for i := 0; i < len(f.funcs); i++ {
		err := f.runOne(i, nil)
		if err != nil {
			f.err = err
			for j := i + 1; j < len(f.errHandlers); j++ {
				if f.errHandlers[j] != nil {
					f.errHandlers[j](f)
				}
			}
			break
		}
	}
	close(f.c)
}

func (f *Future) runParallel() {
	rc := make(chan error, len(f.funcs))
	for i := 0; i < len(f.funcs); i++ {
		go f.runOne(i, rc)
	}
	errs := make([]error, len(f.funcs))
	for i := 0; i < len(f.funcs); i++ {
		errs[i] = <-rc
	}
	for i, err := range errs {
		if err != nil {
			f.err = err
			if f.errHandlers[i] != nil {
				f.errHandlers[i](f)
			}
		}
	}
	close(f.c)
}

func (f *Future) Go() *Future {
	if f.c != nil && !f.IsDone() {
		panic("Go called on running future")
	}
	f.err = nil
	f.c = make(chan struct{})
	go f.run()
	return f
}

func (f *Future) GoParallel() *Future {
	if f.c != nil && !f.IsDone() {
		panic("GoParallel called on running future")
	}
	f.err = nil
	f.c = make(chan struct{})
	go f.runParallel()
	return f
}

func (f *Future) Wait() error {
	if f.c == nil {
		f.Go()
	}
	<-f.c
	return f.err
}

func (f *Future) SetResult(result interface{}) {
	f.result = result
}

func (f *Future) WaitResult() (interface{}, error) {
	err := f.Wait()
	return f.result, err
}

func (f *Future) Result() interface{} {
	return f.result
}

func (f *Future) IsDone() bool {
	if f.c == nil {
		panic("isDone called on uniniitalized future")
	}
	select {
	case <-f.c:
		return true
	default:
		return false
	}
}

func (f *Future) IsComplete() bool {
	if f.c == nil {
		f.Go()
	}
	return f.IsDone()
}

func (f *Future) TimedWait(timeout time.Duration) (bool, error) {
	if f.IsComplete() {
		return true, f.err
	}
	select {
	case <-f.c:
		return true, f.err
	case <-time.After(timeout):
		return false, nil
	}
}

func (f *Future) Then(fn futureFunc) *Future {
	f.addFunc(fn)
	return f
}

func (f *Future) OnError(eh errHandler) *Future {
	if f.errHandlers[len(f.errHandlers)-1] != nil {
		panic("error handler already set")
	}
	f.errHandlers[len(f.errHandlers)-1] = eh
	return f
}

func (f *Future) OnComplete(ch completionFunc) *Future {
	if f.doneHandlers[len(f.funcs)-1] != nil {
		panic("completion handler already set")
	}
	f.doneHandlers[len(f.funcs)-1] = ch
	return f
}

func (f *Future) Err() error {
	if f.c == nil {
		f.Go()
	}
	return f.err
}

func (f *Future) Done() chan struct{} {
	if f.c == nil {
		f.Go()
	}
	return f.c
}

func (f *Future) Cancel() {
	if f.c != nil {
		close(f.c)
	}
}
