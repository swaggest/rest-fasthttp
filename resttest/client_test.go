package resttest_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/swaggest/rest/resttest"
)

func TestNewClient(t *testing.T) {
	cnt := int64(0)
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/foo?q=1", r.URL.String())
		b, err := ioutil.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Equal(t, `{"foo":"bar"}`, string(b))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "abc", r.Header.Get("X-Header"))
		assert.Equal(t, "def", r.Header.Get("X-Custom"))

		ncnt := atomic.AddInt64(&cnt, 1)
		rw.Header().Set("Content-Type", "application/json")
		if ncnt > 1 {
			rw.WriteHeader(http.StatusConflict)
			_, err := rw.Write([]byte(`{"error":"conflict"}`))
			assert.NoError(t, err)
		} else {
			rw.WriteHeader(http.StatusAccepted)
			_, err := rw.Write([]byte(`{"bar":"foo"}`))
			assert.NoError(t, err)
		}
	}))

	defer srv.Close()

	c := resttest.NewClient(srv.URL)
	c.ConcurrencyLevel = 50
	c.Headers = map[string]string{
		"X-Header": "abc",
	}

	c.Reset().
		WithMethod(http.MethodPost).
		WithHeader("X-Custom", "def").
		WithContentType("application/json").
		WithBody([]byte(`{"foo":"bar"}`)).
		WithPath("/foo?q=1").
		Concurrently()

	assert.NoError(t, c.ExpectResponseStatus(http.StatusAccepted))
	assert.NoError(t, c.ExpectResponseBody([]byte(`{"bar":"foo"}`)))
	assert.NoError(t, c.ExpectOtherResponsesStatus(http.StatusConflict))
	assert.NoError(t, c.ExpectOtherResponsesBody([]byte(`{"error":"conflict"}`)))
}