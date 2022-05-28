package gistova

import (
	"context"
	"fmt"
	"time"
)

// PayloadRunningFunc is a func that takes a pointer to a Payload as an
// argument and returns an error (or nil, on success).
type PayloadRunningFunc func(context.Context, *Payload) error

// A PayloadRunner is a type that offers a Run method accepting a pointer
// to a Payload returning an error (or nil, on success).
type PayloadRunner interface {
	Run(context.Context, *Payload) error
}

type funcRunner PayloadRunningFunc

func (fr funcRunner) Run(ctx context.Context, p *Payload) error {
	return fr(ctx, p)
}

// TryFunc wraps prf inside of a PayloadRunner and runs p.Try() on it.
func (p *Payload) TryFunc(prf PayloadRunningFunc) error {
	return p.Try(funcRunner(prf))
}

// Try runs a Payload through the provided PayloadRunner. It provides Run with
// the appropriate Context and guards against any panics. pr.Run() has the option
// of calling p.Respond/p.Fail to explicitly notify the runtime, or it may return
// without calling them, in which case a nil return indicates success (with no
// provided response data) and a non-nil return indicates failure.
func (p *Payload) Try(pr PayloadRunner) (err error) {
	defer func() {
		if pv := recover(); pv != nil {
			if perr, isErr := pv.(error); isErr {
				// optimistically wrap errors when possible
				err = fmt.Errorf("%T error: %w", pr, perr)
			} else {
				err = fmt.Errorf("%T panic: %v", pr, pv)
			}
			if p.done {
				// if a response has already been sent, nothing else to do
				return
			}
			// send failure to the Runtime, and recover gracefully
			err = p.FailWithError("function invocation panicked", err)
		}
	}()
	err = pr.Run(p.Context, p)
	if p.done {
		// pr.Run() sent its own response already
		return
	}
	if err == nil {
		// pr.Run() didn't fail and didn't respond; send an empty success to the Runtime
		err = p.Respond(nil)
		return
	}
	err = p.FailWithError("function invocation failed", err)
	return
}

// Loop requests Payload p and runs p.Try(pr) forever. It does not return. Specify a
// Logger in l or give nil for a default Logger.
func (r *Runtime) Loop(pr PayloadRunner, l Logger) {
	var p Payload
	var err error
	var count uint64
	var trystart time.Time
	ctx := context.Background()
	if l == nil {
		l = DefaultLogger()
	}
	l.Logln("Started gistova event loop")
	p.Logger = l
	for {
		err = r.Wait(ctx, &p)
		if err != nil {
			l.Errorf("Error attempting to fetch payload: %v", err)
			r.backoff(l)
			continue
		}
		count++
		l.Logf("Invocation #%d, request id %s started", count, p.RequestID)
		trystart = time.Now()
		err = p.Try(pr)
		l.Logf("Invocation #%d, request id %s complete (%s)", count, p.RequestID, time.Since(trystart))
		if err != nil {
			l.Errorf("Unhandled function error: %v", err)
			r.backoff(l)
		}
	}
}

// LoopFunc creates a PayloadRunner from prf and runs Loop. It does not return. Specify a
// Logger in l or give nil for a default Logger.
func (r *Runtime) LoopFunc(prf PayloadRunningFunc, l Logger) {
	r.Loop(funcRunner(prf), l)
}
