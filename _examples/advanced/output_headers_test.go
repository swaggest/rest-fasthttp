package main

import (
	"net/http"
	"testing"

	"github.com/bool64/httptestbench"
	"github.com/swaggest/fchi"
	"github.com/valyala/fasthttp"
)

// Benchmark_outputHeaders-4   	   41424	     27054 ns/op	       154 B:rcvd/op	        77.0 B:sent/op	     36963 rps	    3641 B/op	      35 allocs/op.
func Benchmark_outputHeaders(b *testing.B) {
	r := NewRouter()

	srv := fchi.NewTestServer(r)
	defer srv.Close()

	httptestbench.RoundTrip(b, 50, func(i int, req *fasthttp.Request) {
		req.Header.SetMethod(http.MethodGet)
		req.SetRequestURI(srv.URL + "/output-headers")
	}, func(i int, resp *fasthttp.Response) bool {
		return resp.StatusCode() == http.StatusOK
	})
}
