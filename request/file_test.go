package request_test

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/swaggest/fchi"
	"github.com/swaggest/rest"
	"github.com/swaggest/rest-fasthttp/chirouter"
	"github.com/swaggest/rest-fasthttp/fhttp"
	"github.com/swaggest/rest-fasthttp/request"
	"github.com/swaggest/rest-fasthttp/response"
	"github.com/swaggest/rest/jsonschema"
	"github.com/swaggest/rest/openapi"
	"github.com/swaggest/usecase"
)

type ReqEmb struct {
	UploadHeader   *multipart.FileHeader   `formData:"upload"`
	UploadsHeaders []*multipart.FileHeader `formData:"uploads"`
}

type fileReqTest struct {
	ReqEmb
	Upload  multipart.File   `file:"upload"`
	Uploads []multipart.File `formData:"uploads"`
}

func TestMapper_Decode_fileUploadTag(t *testing.T) {
	r := chirouter.NewWrapper(fchi.NewRouter())
	apiSchema := openapi.Collector{}
	decoderFactory := request.NewDecoderFactory()
	validatorFactory := jsonschema.NewFactory(&apiSchema, &apiSchema)

	decoderFactory.SetDecoderFunc(rest.ParamInPath, chirouter.PathToURLValues)

	r.Wrap(
		fhttp.OpenAPIMiddleware(&apiSchema),
		request.DecoderMiddleware(decoderFactory),
		request.ValidatorMiddleware(validatorFactory),
		response.EncoderMiddleware,
	)

	u := struct {
		usecase.Interactor
		usecase.WithInput
	}{}

	u.Input = new(fileReqTest)
	u.Interactor = usecase.Interact(func(ctx context.Context, input, output interface{}) error {
		in, ok := input.(*fileReqTest)
		assert.True(t, ok)
		assert.NotNil(t, in.Upload)
		assert.NotNil(t, in.UploadHeader)
		assert.Equal(t, "my.csv", in.UploadHeader.Filename)
		assert.Equal(t, int64(6), in.UploadHeader.Size)
		content, err := ioutil.ReadAll(in.Upload)
		assert.NoError(t, err)
		assert.NoError(t, in.Upload.Close())
		assert.Equal(t, "Hello!", string(content))

		assert.Len(t, in.Uploads, 2)
		assert.Len(t, in.UploadsHeaders, 2)

		if !assert.Len(t, in.Uploads, 2) || assert.Len(t, in.UploadsHeaders, 2) {
			return errors.New("missing uploads")
		}

		assert.Equal(t, "my1.csv", in.UploadsHeaders[0].Filename)
		assert.Equal(t, int64(7), in.UploadsHeaders[0].Size)
		assert.Equal(t, "my2.csv", in.UploadsHeaders[1].Filename)
		assert.Equal(t, int64(7), in.UploadsHeaders[1].Size)

		content, err = ioutil.ReadAll(in.Uploads[0])
		assert.NoError(t, err)
		assert.NoError(t, in.Uploads[0].Close())
		assert.Equal(t, "Hello1!", string(content))

		content, err = ioutil.ReadAll(in.Uploads[1])
		assert.NoError(t, err)
		assert.NoError(t, in.Uploads[1].Close())
		assert.Equal(t, "Hello2!", string(content))

		return nil
	})

	h := fhttp.NewHandler(u)
	r.Method(http.MethodPost, "/receive", h)

	srv := fchi.NewTestServer(r)
	defer srv.Close()

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	writer, err := w.CreateFormFile("upload", "my.csv")
	assert.NoError(t, err)

	_, err = writer.Write([]byte(`Hello!`))
	assert.NoError(t, err)

	writer, err = w.CreateFormFile("uploads", "my1.csv")
	assert.NoError(t, err)

	_, err = writer.Write([]byte(`Hello1!`))
	assert.NoError(t, err)

	writer, err = w.CreateFormFile("uploads", "my2.csv")
	assert.NoError(t, err)

	_, err = writer.Write([]byte(`Hello2!`))
	assert.NoError(t, err)

	assert.NoError(t, w.Close())

	hreq := httptest.NewRequest(http.MethodPost, srv.URL+"/receive", &b)
	hreq.RequestURI = ""
	hreq.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := http.DefaultTransport.RoundTrip(hreq)
	assert.NoError(t, err)
	assert.NoError(t, resp.Body.Close())
}
