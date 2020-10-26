package jsonschema_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/swaggest/rest"
	"github.com/swaggest/rest/jsonschema"
	"github.com/swaggest/rest/openapi"
)

// BenchmarkRequestValidator_ValidateRequestData-4   	  634356	      1761 ns/op	    2496 B/op	       8 allocs/op.
func BenchmarkRequestValidator_ValidateRequestData(b *testing.B) {
	validator := jsonschema.NewFactory(&openapi.Collector{}, &openapi.Collector{}).
		MakeRequestValidator(http.MethodPost, new(struct {
			Cookie string `cookie:"in_cookie" minLength:"3" required:"true"`
		}), nil)

	b.ResetTimer()
	b.ReportAllocs()

	value := map[string]interface{}{
		"in_cookie": "abc",
	}

	for i := 0; i < b.N; i++ {
		err := validator.ValidateData(rest.ParamInCookie, value)
		if err != nil {
			b.Fail()
		}
	}
}

func TestRequestValidator_ValidateData(t *testing.T) {
	validator := jsonschema.NewFactory(&openapi.Collector{}, &openapi.Collector{}).
		MakeRequestValidator(http.MethodPost, new(struct {
			Cookie string `cookie:"in_cookie" minimum:"100" required:"true"`
			Query  string `query:"in_query_ignored" minLength:"3"`
		}), rest.RequestMapping{
			rest.ParamInQuery: map[string]string{"Query": "in_query"},
		})

	err := validator.ValidateData(rest.ParamInCookie, map[string]interface{}{"in_cookie": 123})
	assert.Equal(t, err, rest.ValidationErrors{"cookie:in_cookie": []string{"#: expected string, but got number"}})

	err = validator.ValidateData(rest.ParamInCookie, map[string]interface{}{})
	assert.Equal(t, err, rest.ValidationErrors{"cookie:in_cookie": []string{"missing value"}})

	err = validator.ValidateData(rest.ParamInQuery, map[string]interface{}{"in_query": 123})
	assert.Equal(t, err, rest.ValidationErrors{"query:in_query": []string{"#: expected string, but got number"}})

	err = validator.ValidateData(rest.ParamInQuery, map[string]interface{}{"in_query": "ab"})
	assert.Equal(t, err, rest.ValidationErrors{"query:in_query": []string{"#: length must be >= 3, but got 2"}})

	assert.NoError(t, validator.ValidateData(rest.ParamInQuery, map[string]interface{}{}))
	assert.NoError(t, validator.ValidateData(rest.ParamInQuery, map[string]interface{}{"unknown": 123}))
	assert.NoError(t, validator.ValidateData(rest.ParamInQuery, map[string]interface{}{"in_query_ignored": 123}))
	assert.NoError(t, validator.ValidateData("unknown", map[string]interface{}{}))
	assert.NoError(t, validator.ValidateData(rest.ParamInCookie, map[string]interface{}{"in_cookie": "abc"}))
}