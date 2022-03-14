package gistova

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// A Payload represents a single invocation and provides methods to deliver
// a response to the Runtime API with the result.
// Payload is intended to be reused.
type Payload struct {
	runtime    *Runtime
	done       bool
	Context    context.Context
	Cancel     context.CancelFunc
	RequestID  string
	InvokedARN string
	TraceID    string
	JSON       bytes.Buffer
}

// Reset makes a Payload ready to reuse.
func (p *Payload) Reset() {
	if p.Cancel != nil {
		p.Cancel()
		p.Cancel = nil
	}
	p.JSON.Reset()
	p.RequestID = ""
	p.Context = nil
	p.done = false
}

func (p *Payload) url(path string) *url.URL {
	if p.runtime == nil {
		panic("Payload.url() called with nil p.runtime")
	}
	if p.RequestID == "" {
		panic("Payload.url() called with empty p.RequestID")
	}
	u, err := url.Parse(p.runtime.endpoint + "/invocation/" + p.RequestID + path)
	if err != nil {
		panic("Payload.url() called but unable to parse URL: " + err.Error())
	}
	return u
}

func (p *Payload) send(path string, hdr http.Header, body io.Reader) error {
	if p.done {
		return fmt.Errorf("payload is marked as done, cannot send another response")
	}
	if hdr == nil {
		hdr = http.Header{}
	}
	bodyCloser, bodyLen := bodyWrap(body)
	res, err := p.runtime.respclient.Do(&http.Request{
		Method:        "POST",
		URL:           p.url(path),
		Header:        hdr,
		Body:          bodyCloser,
		ContentLength: bodyLen,
	})
	if err != nil {
		return fmt.Errorf("unable to send response: %w", err)
	}
	p.done = true
	res.Body.Close()
	if err := httpStatusErr(res); err != nil {
		return fmt.Errorf("runtime rejected response: %w", err)
	}
	return nil
}

// Respond sends a success response to the Runtime for this Payload. If
// d implements Close(), it will be called when the response is sent. If
// d implements Len(), it will be used to improve transfer efficiency.
func (p *Payload) Respond(d io.Reader) error {
	return p.send("/response", nil, d)
}

// RespondBytes sends a success response to the Runtime, wrapping the
// provided slice of bytes in a bytes.Reader.
func (p *Payload) RespondBytes(b []byte) error {
	return p.Respond(bytes.NewReader(b))
}

// RespondString sends a success response to the Runtime, wrapping the
// provided string in a strings.Reader.
func (p *Payload) RespondString(s string) error {
	return p.Respond(strings.NewReader(s))
}

// Fail sends an error response to the Runtime for this Payload. Arguments
// are optional, at your peril.
func (p *Payload) Fail(errorType string, detail *Failure) error {
	reqh := http.Header{}
	if errorType != "" {
		reqh.Set("Lambda-Runtime-Function-Error-Type", errorType)
	}
	var body io.Reader
	if detail != nil {
		j, err := json.Marshal(detail)
		if err != nil {
			return fmt.Errorf("unable to create failure response: %w", err)
		}
		body = bytes.NewReader(j)
	}
	return p.send("/error", reqh, body)
}

// FailWithError sends an error response to the Runtime based on the
// error value e and the descriptive text desc. desc is optional.
func (p *Payload) FailWithError(desc string, e error) error {
	j := &Failure{}
	j.Type = fmt.Sprintf("%T", e)
	if desc != "" {
		j.Message = fmt.Sprintf("%s: %v", desc, e)
	} else {
		j.Message = fmt.Sprintf("%v", e)
	}
	return p.Fail(j.Type, j)
}
