package stutil

import (
	"bytes"
	"crypto/tls"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/cookiejar"
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
	if req.urlValue == nil {
		req.urlValue = make(url.Values)
	}
	req.urlValue.Set(k, v)
	return req
}

//params	post form的数据
//nameField	请求地址上传文件对应field
//fileName	文件名
//file	文件
func (req *HttpRequest) FormFile(params map[string]string, nameField, fileName string, file io.Reader) (*HttpRequest, error) {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	formFile, err := writer.CreateFormFile(nameField, fileName)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(formFile, file)
	if err != nil {
		return nil, err
	}

	for key, val := range params {
		_ = writer.WriteField(key, val)
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req.binData = body.Bytes()
	req.Header("Content-Type", writer.FormDataContentType())
	//req.Header.Add("Content-Type", writer.FormDataContentType())

	return req, nil
}

func (req *HttpRequest) FormJson(s string) *HttpRequest {
	req.Header("Content-Type", HTTPJSONContentType)
	if len(s) == 0 {
		req.binData = nil
	} else {
		req.binData = []byte(s)
	}

	return req
}

func (req *HttpRequest) Proxy(p string) *HttpRequest {
	req.proxy = p
	return req
}

func (req *HttpRequest) SkipVerify() *HttpRequest {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	jar, _ := cookiejar.New(nil)
	req.client = &http.Client{Transport: tr, Jar: jar}
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
		resp.Body.Close()
	}
	return
}

func (req *HttpRequest) Post(sUrl string) (resp *http.Response, body []byte, err error) {
	return req.Do("POST", sUrl)
}

func (req *HttpRequest) Get(sUrl string) (resp *http.Response, body []byte, err error) {
	return req.Do("GET", sUrl)
}

func (req *HttpRequest) build(method string, sUrl string) (request *http.Request, err error) {
	if len(req.binData) != 0 {
		request, err = http.NewRequest(method, sUrl, bytes.NewBuffer(req.binData))
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
		jar, err := cookiejar.New(nil)
		if err != nil {
			return nil, err
		}
		req.client.Jar = jar
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
