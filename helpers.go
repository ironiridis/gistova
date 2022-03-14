package gistova

import (
	"fmt"
	"io"
	"net/http"
)

func bodyWrap(d io.Reader) (rc io.ReadCloser, l int64) {
	if d == nil {
		// nil means no body, so return nil rc and zero length
		return
	}
	// if the underlying type has Close, use it
	rc, ok := d.(io.ReadCloser)
	if !ok {
		// otherwise wrap with NopCloser
		rc = io.NopCloser(d)
	}
	// does the underlying type know how much data it has?
	type hasLen interface{ Len() int }
	if dl, ok := d.(hasLen); ok {
		// if so, return that
		l = int64(dl.Len())
	} else {
		// if not, return -1; http.Request.Write treats -1 as an unknown length
		l = -1
	}
	return
}

func httpStatusErr(r *http.Response) error {
	switch {
	case r.StatusCode >= 500:
		return fmt.Errorf("runtime API returned server error: %q", r.Status)
	case r.StatusCode >= 400:
		return fmt.Errorf("runtime API returned client error: %q", r.Status)
	case r.StatusCode >= 300, r.StatusCode < 200:
		return fmt.Errorf("runtime API returned unexpected HTTP %q", r.Status)
	}
	return nil
}
