package middleware_test

import (
	"bufio"
	"bytes"
	"io"
	"net/http"
	"testing"

	"github.com/valyala/fasthttp"
	"github.com/valyala/fasthttp/fasthttputil"
)

// dispatchFast wraps next with mw, runs a synthetic request through fasthttp
// via an in-memory listener, and returns the parsed HTTP response.
//
// Replaces httptest.NewRecorder for fasthttp middleware tests.
func dispatchFast(t *testing.T, mw func(fasthttp.RequestHandler) fasthttp.RequestHandler, req *http.Request, downstream fasthttp.RequestHandler) (*http.Response, *bytes.Buffer) {
	t.Helper()

	if downstream == nil {
		downstream = func(ctx *fasthttp.RequestCtx) {
			ctx.SetStatusCode(fasthttp.StatusOK)
		}
	}
	handler := mw(downstream)

	ln := fasthttputil.NewInmemoryListener()
	srv := &fasthttp.Server{Handler: handler}
	srvErr := make(chan error, 1)
	go func() { srvErr <- srv.Serve(ln) }()
	t.Cleanup(func() {
		_ = ln.Close()
		<-srvErr
	})

	c, err := ln.Dial()
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = c.Close() }()

	// Build the wire request. fasthttp accepts a plain HTTP/1.1 request on
	// the inmemory pipe.
	var buf bytes.Buffer
	if req.Host == "" {
		req.Host = "localhost"
	}
	if err := req.Write(&buf); err != nil {
		t.Fatalf("write req: %v", err)
	}
	if _, err := c.Write(buf.Bytes()); err != nil {
		t.Fatalf("net write: %v", err)
	}

	respBuf := bufio.NewReader(c)
	resp, err := http.ReadResponse(respBuf, req)
	if err != nil {
		t.Fatalf("read resp: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return resp, bytes.NewBuffer(body)
}
