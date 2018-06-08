package stutil

import (
	"bytes"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// HTTP content type
var (
	DefaultContentType  = "application/x-www-form-urlencoded; charset=utf-8"
	HTTPFORMContentType = "application/x-www-form-urlencoded"
	HTTPJSONContentType = "application/json"
	HTTPXMLContentType  = "text/xml"
	HTTPFILEContentType = "multipart/form-data"
)

// DefaultHeaders define default headers
var DefaultHeaders = map[string]string{
	"Connection": "keep-alive",
	"Accept":     "*/*",
	"User-Agent": "Chrome",
}

type HttpRequest struct {
	client      *http.Client
	headers     map[string]string
	urlValue    url.Values // Sent by form data
	binData     []byte     // suit for POSTJSON(), POSTFILE()
	contentType string
	proxy       string
}

func (req *HttpRequest) Header(key, val string) *HttpRequest {
	if req.headers == nil {
		req.headers = make(map[string]string, 1)
	}
	req.headers[key] = val
	return req
}

func (req *HttpRequest) Cookie(cookie string) *HttpRequest {
	if req.headers == nil {
		req.headers = make(map[string]string, 1)
	}
	cookie = strings.Replace(cookie, " ", "", -1)
	cookie = strings.Replace(cookie, "\n", "", -1)
	cookie = strings.Replace(cookie, "\r", "", -1)
	req.headers["Cookie"] = cookie
	return req
}

func (req *HttpRequest) FormParm(k, v string) *HttpRequest {
	req.urlValue.Set(k, v)
	return req
}

func (req *HttpRequest) Proxy(p string) *HttpRequest {
	req.proxy = p
	return req
}

func (req *HttpRequest) Do(method string, sUrl string) (resp *http.Response, body []byte, err error) {
	request, e := req.build(method, sUrl)
	if e != nil {
		return nil, nil, e
	}
	resp, err = req.client.Do(request)
	if err != nil {
		return nil, nil, err
	}

	if resp != nil {
		body, err = ioutil.ReadAll(resp.Body)
		//response.Close = true
		defer resp.Body.Close()
		if err != nil {
			return nil, nil, err
		}
	}

	return
}

func (req *HttpRequest) build(method string, sUrl string) (request *http.Request, err error) {
	if len(req.binData) != 0 {
		pr := ioutil.NopCloser(bytes.NewReader(req.binData))
		request, err = http.NewRequest(method, sUrl, pr)
	} else if len(req.urlValue) != 0 {
		pr := ioutil.NopCloser(strings.NewReader(req.urlValue.Encode()))
		request, err = http.NewRequest(method, sUrl, pr)
	} else {
		request, err = http.NewRequest(method, sUrl, nil)
	}
	if err != nil {
		return nil, err
	}

	for k, v := range DefaultHeaders {
		_, ok := req.headers[k]
		if !ok {
			request.Header.Set(k, v)
		}
	}
	for k, v := range req.headers {
		request.Header.Set(k, v)
	}
	_, ok := req.headers["Content-Type"]
	if !ok {
		if req.contentType != "" {
			request.Header.Set("Content-Type", req.contentType)
		} else {
			request.Header.Set("Content-Type", DefaultContentType)
		}
	}

	if req.client == nil {
		req.client = new(http.Client)
	}
	if req.proxy != "" {
		u, e := url.Parse(req.proxy)
		if e != nil {
			return nil, e
		}
		switch u.Scheme {
		case "http", "https":
			req.client.Transport = &http.Transport{
				Proxy: http.ProxyURL(u),
				Dial: (&net.Dialer{
					Timeout: 30 * time.Second,
					// KeepAlive: 30 * time.Second,
				}).Dial,
				// TLSHandshakeTimeout: 10 * time.Second,
			}
		case "socks5":
			dialer, err := proxy.FromURL(u, proxy.Direct)
			if err != nil {
				return nil, err
			}
			req.client.Transport = &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				Dial:  dialer.Dial,
				// TLSHandshakeTimeout: 10 * time.Second,
			}
		}
	}

	return
}
