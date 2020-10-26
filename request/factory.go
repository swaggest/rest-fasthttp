package request

import (
	"net/http"
	"net/url"
	"reflect"
	"strings"

	"github.com/swaggest/form"
	"github.com/swaggest/openapi-go/openapi3"
	"github.com/swaggest/refl"
	"github.com/swaggest/rest"
	"github.com/swaggest/rest/nethttp"
)

var _ DecoderMaker = &DecoderFactory{}

const (
	defaultTag  = "default"
	jsonTag     = "json"
	fileTag     = "file"
	formDataTag = "formData"
)

// DecoderFactory decodes http requests.
//
// Please use NewDecoderFactory to create instance.
type DecoderFactory struct {
	ApplyDefaults bool

	formDecoders      map[rest.ParamIn]*form.Decoder
	decoderFunctions  map[rest.ParamIn]decoderFunc
	defaultValDecoder *form.Decoder
	customDecoders    []customDecoder
}

type customDecoder struct {
	types []interface{}
	fn    form.DecodeFunc
}

// NewDecoderFactory creates request decoder factory.
func NewDecoderFactory() *DecoderFactory {
	df := DecoderFactory{}
	df.SetDecoderFunc(rest.ParamInCookie, cookiesToURLValues)
	df.SetDecoderFunc(rest.ParamInFormData, formDataToURLValues)
	df.SetDecoderFunc(rest.ParamInHeader, headerToURLValues)
	df.SetDecoderFunc(rest.ParamInQuery, queryToURLValues)

	defaultValDecoder := form.NewDecoder()
	defaultValDecoder.RegisterTagNameFunc(func(field reflect.StructField) string {
		return field.Name
	})

	df.defaultValDecoder = defaultValDecoder

	return &df
}

// SetDecoderFunc adds custom decoder function for values of particular field tag name.
func (df *DecoderFactory) SetDecoderFunc(tagName rest.ParamIn, d func(r *http.Request) (url.Values, error)) {
	if df.decoderFunctions == nil {
		df.decoderFunctions = make(map[rest.ParamIn]decoderFunc)
	}

	if df.formDecoders == nil {
		df.formDecoders = make(map[rest.ParamIn]*form.Decoder)
	}

	df.decoderFunctions[tagName] = d
	dec := form.NewDecoder()
	dec.SetTagName(string(tagName))
	dec.SetMode(form.ModeExplicit)
	df.formDecoders[tagName] = dec
}

// MakeDecoder creates request.RequestDecoder for a http method and request structure.
//
// Input is checked for `json`, `file` tags only for methods with body semantics (POST, PUT, PATCH) or
// if input implements openapi3.RequestBodyEnforcer.
//
// CustomMapping can be nil, otherwise it is used instead of field tags to match decoded fields with struct.
func (df *DecoderFactory) MakeDecoder(
	method string,
	input interface{},
	customMapping rest.RequestMapping,
) nethttp.RequestDecoder {
	m := decoder{
		decoders: make([]valueDecoderFunc, 0),
		in:       make([]rest.ParamIn, 0),
	}

	if df.ApplyDefaults && refl.HasTaggedFields(input, defaultTag) {
		df.makeDefaultDecoder(input, &m)
	}

	if len(customMapping) > 0 {
		df.makeCustomMappingDecoder(customMapping, &m)
	}

	for in, formDecoder := range df.formDecoders {
		if _, exists := customMapping[in]; exists {
			continue
		}

		if refl.HasTaggedFields(input, string(in)) {
			// dfu :=  df.decoderFunctions[in]
			m.decoders = append(m.decoders, makeDecoder(in, formDecoder, df.decoderFunctions[in]))
			m.in = append(m.in, in)
		}
	}

	method = strings.ToUpper(method)

	_, forceRequestBody := input.(openapi3.RequestBodyEnforcer)

	if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch && !forceRequestBody {
		return &m
	}

	// Checking for body tags.
	if refl.HasTaggedFields(input, jsonTag) || refl.FindEmbeddedSliceOrMap(input) != nil {
		m.decoders = append(m.decoders, decodeJSONBody)
		m.in = append(m.in, rest.ParamInBody)
	}

	if hasFileFields(input, fileTag) || hasFileFields(input, formDataTag) {
		m.decoders = append(m.decoders, decodeFiles)
		m.in = append(m.in, rest.ParamInFormData)
	}

	return &m
}

func (df *DecoderFactory) makeDefaultDecoder(input interface{}, m *decoder) {
	defaults := url.Values{}

	refl.WalkTaggedFields(reflect.ValueOf(input), func(v reflect.Value, sf reflect.StructField, tag string) {
		defaults[sf.Name] = []string{tag}
	}, defaultTag)

	dec := df.defaultValDecoder

	m.decoders = append(m.decoders, func(r *http.Request, v interface{}, validator rest.Validator) error {
		return dec.Decode(v, defaults)
	})
}

func (df *DecoderFactory) makeCustomMappingDecoder(customMapping rest.RequestMapping, m *decoder) {
	for in, mapping := range customMapping {
		dec := form.NewDecoder()
		dec.SetTagName(string(in))

		// Copy mapping to avoid mutability.
		mm := make(map[string]string, len(mapping))
		for k, v := range mapping {
			mm[k] = v
		}

		dec.RegisterTagNameFunc(func(field reflect.StructField) string {
			return mm[field.Name]
		})

		for _, c := range df.customDecoders {
			dec.RegisterFunc(c.fn, c.types...)
		}

		m.decoders = append(m.decoders, makeDecoder(in, dec, df.decoderFunctions[in]))
		m.in = append(m.in, in)
	}
}

// RegisterFunc adds custom type handling.
func (df *DecoderFactory) RegisterFunc(fn form.DecodeFunc, types ...interface{}) {
	for _, fd := range df.formDecoders {
		fd.RegisterFunc(fn, types...)
	}

	df.defaultValDecoder.RegisterFunc(fn, types...)

	df.customDecoders = append(df.customDecoders, customDecoder{
		fn:    fn,
		types: types,
	})
}