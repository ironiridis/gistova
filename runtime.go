package gistova

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Failure holds information about a failure to supply to the Runtime.
type Failure struct {
	Message string   `json:"errorMessage"`
	Type    string   `json:"errorType"`
	Trace   []string `json:"stackTrace"`
}

// A Runtime provides a way to request Payloads from the Lambda Runtime API.
type Runtime struct {
	endpoint   string
	failcount  uint
	failwait   time.Duration
	faillast   time.Time
	waitclient *http.Client
	respclient *http.Client
}

// DefaultRuntime returns an initialized Runtime.
func DefaultRuntime() *Runtime {
	r := &Runtime{}

	if host, ok := os.LookupEnv("AWS_LAMBDA_RUNTIME_API"); ok {
		r.endpoint = fmt.Sprintf("http://%s/2018-06-01/runtime", host)
	} else {
		panic("AWS_LAMBDA_RUNTIME_API not set")
	}

	// http.DefaultTransport imposes some default timeout values
	// So instead, make an HTTP client that disables all timeouts explicitly
	// Note that lots of these values are implicitly 0
	r.waitclient = &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{FallbackDelay: -1}).DialContext,
		},
	}

	// Responses to the lambda runtime should be fast, and fail fast
	// We should also leave open connections for responses
	r.respclient = &http.Client{
		Transport: &http.Transport{
			DialContext:           (&net.Dialer{Timeout: time.Second * 5}).DialContext,
			IdleConnTimeout:       time.Second * 60,
			ResponseHeaderTimeout: time.Second * 5,
		},
		Timeout: time.Second * 30,
	}
	return r
}

// Wait requests a new payload from the Runtime. The Runtime may pause the whole application
// and callers should be prepared for an indefinite delay.
func (r *Runtime) Wait(ctx context.Context, p *Payload) error {
	res, err := r.waitclient.Get(r.endpoint + "/invocation/next")
	if err != nil {
		return fmt.Errorf("error waiting for next invocation: %w", err)
	}
	if err := httpStatusErr(res); err != nil {
		return err
	}
	defer res.Body.Close()

	p.Reset()
	if dl, err := strconv.ParseInt(res.Header.Get("Lambda-Runtime-Deadline-Ms"), 10, 64); err == nil {
		p.Context, p.Cancel = context.WithDeadline(ctx, time.UnixMilli(dl))
	} else {
		return fmt.Errorf("cannot parse deadline: %w", err)
	}
	p.RequestID = res.Header.Get("Lambda-Runtime-Aws-Request-Id")
	p.InvokedARN = res.Header.Get("Lambda-Runtime-Invoked-Function-Arn")
	p.TraceID = res.Header.Get("Lambda-Runtime-Trace-Id")
	_, err = p.JSON.ReadFrom(res.Body)
	if err != nil {
		return fmt.Errorf("unable to read payload body: %w", err)
	}
	p.runtime = r
	return nil
}

func (r *Runtime) backoff(l Logger) {
	// check to see if we can reduce backoff
	if !r.faillast.IsZero() { // do we have a previous timestamp error?
		// if it's been at least 3 backoff delays plus one second...
		if time.Now().After(r.faillast.Add(3*r.failwait + time.Second)) {
			r.failwait /= 2 // reduce backoff delay by half
			r.failcount = 0
			l.Logf("Error backoff delay reduced to %s", r.failwait.String())
		}
	}
	r.failcount++
	r.faillast = time.Now()
	if r.failcount > 3 {
		if r.failwait == 0 {
			r.failwait = time.Millisecond * 50
		} else {
			r.failwait *= 2
		}
		l.Logf("Error backoff delay increased to %s", r.failwait.String())
		time.Sleep(r.failwait)
	}
}
