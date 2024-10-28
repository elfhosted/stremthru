package alldebrid

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/MunifTanjim/stremthru/core"
	"github.com/MunifTanjim/stremthru/store"
)

var DefaultHTTPTransport = func() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DisableKeepAlives = true
	return transport
}()

var DefaultHTTPClient = func() *http.Client {
	return &http.Client{
		Transport: DefaultHTTPTransport,
	}
}()

type APIClientConfig struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	agent      string
}

type APIClient struct {
	BaseURL    *url.URL
	HTTPClient *http.Client
	apiKey     string
	agent      string
}

func NewAPIClient(conf *APIClientConfig) *APIClient {
	if conf.agent == "" {
		conf.agent = "stremthru"
	}

	if conf.BaseURL == "" {
		conf.BaseURL = "https://api.alldebrid.com"
	}

	if conf.HTTPClient == nil {
		conf.HTTPClient = DefaultHTTPClient
	}

	c := &APIClient{}

	baseUrl, err := url.Parse(conf.BaseURL)
	if err != nil {
		panic(err)
	}

	c.BaseURL = baseUrl
	c.HTTPClient = conf.HTTPClient
	c.apiKey = conf.APIKey
	c.agent = conf.agent

	return c
}

type RequestContext interface {
	getContext() context.Context
	getBody(method string, query *url.Values) (body io.Reader, contentType string)
	setAuthHeader(req *http.Request, apiKey string)
}

type Ctx struct {
	APIKey  string
	Context context.Context
	Form    *url.Values
}

func (rc Ctx) getContext() context.Context {
	if rc.Context == nil {
		rc.Context = context.Background()
	}
	return rc.Context
}

func (rc Ctx) getBody(method string, query *url.Values) (body io.Reader, contentType string) {
	if rc.Form != nil {
		if method == http.MethodHead || method == http.MethodGet {
			for key, values := range *rc.Form {
				for _, value := range values {
					query.Add(key, value)
				}
			}
		} else {
			body = strings.NewReader(rc.Form.Encode())
			contentType = "application/x-www-form-urlencoded"
		}
	}
	return body, contentType
}

func (rc Ctx) setAuthHeader(req *http.Request, apiKey string) {
	if len(rc.APIKey) > 0 {
		apiKey = rc.APIKey
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
}

func (c APIClient) newRequest(method, path string, params RequestContext) (req *http.Request, err error) {
	if params == nil {
		params = &Ctx{}
	}

	url := c.BaseURL.JoinPath(path)

	query := url.Query()
	query.Set("agent", c.agent)

	body, contentType := params.getBody(method, &query)

	url.RawQuery = query.Encode()

	req, err = http.NewRequestWithContext(params.getContext(), method, url.String(), body)
	if err != nil {
		return nil, err
	}

	params.setAuthHeader(req, c.apiKey)
	req.Header.Add("User-Agent", c.agent)
	if len(contentType) > 0 {
		req.Header.Add("Content-Type", contentType)
	}

	return req, nil
}

func (c APIClient) Request(method, path string, params RequestContext, v ResponseEnvelop) (*http.Response, error) {
	req, err := c.newRequest(method, path, params)
	if err != nil {
		error := core.NewStoreError("failed to create request")
		error.StoreName = string(store.StoreNameAlldebrid)
		error.Cause = err
		return nil, error
	}
	res, err := c.HTTPClient.Do(req)
	err = processResponseBody(res, err, v)
	if err != nil {
		return res, UpstreamErrorFromRequest(err, req, res)
	}
	return res, nil
}
