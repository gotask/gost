package stnet

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"net/http"
	"net/textproto"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type httpWriter struct {
	current       *CurrentContent
	header        http.Header
	status        int
	isWroteHeader bool
	buf           bytes.Buffer
}

func (w *httpWriter) Header() http.Header {
	return w.header
}
func (w *httpWriter) Write(b []byte) (int, error) {
	if !w.isWroteHeader { //WriteHeader
		if _, ok := w.header["Content-Length"]; !ok {
			w.header.Set("Content-Length", strconv.Itoa(len(b)))
		}
		w.WriteHeader(200)
	}

	if w.buf.Len() > 0 {
		if len(w.header) == 0 {
			fmt.Fprintf(&w.buf, "%s: %d\r\n", "Content-Length", len(b))
		}
		w.buf.WriteString("\r\n") //http head end
		w.buf.Write(b)            //send header and body
		e := w.current.Sess.Send(w.buf.Bytes())
		w.buf.Reset()
		return len(b), e
	}

	//send body
	e := w.current.Sess.Send(b)
	return len(b), e
}
func (w *httpWriter) WriteHeader(statusCode int) {
	w.status = statusCode

	buf := &w.buf

	if w.status == 0 {
		w.status = 200 //ok
	}
	text := http.StatusText(w.status)
	if text == "" {
		text = "status code " + strconv.Itoa(w.status)
	}
	fmt.Fprintf(buf, "HTTP/1.1 %03d %s\r\n", w.status, text)

	for k, ss := range w.header {
		for _, s := range ss {
			v := strings.Replace(s, "\n", " ", -1)
			fmt.Fprintf(buf, "%s: %s\r\n", k, v)
		}
	}

	w.isWroteHeader = true

	// wrong status code
	if (w.status >= 100 && w.status <= 199) || w.status == 204 || w.status == 304 {
		w.Write(nil)
	}
}

type HttpService interface {
	Init() bool
	Loop()
	HandleError(current *CurrentContent, e error)
	HashProcessor(current *CurrentContent, req *http.Request) (processorID int)
}

type ServiceHttp struct {
	ServiceBase
	imp HttpService
	h   *HttpHandler
}

func (service *ServiceHttp) Init() bool {
	return service.imp.Init()
}

func (service *ServiceHttp) Loop() {
	service.imp.Loop()
}

func (service *ServiceHttp) HandleMessage(current *CurrentContent, msgID uint64, msg interface{}) {
	r := msg.(*http.Request)
	h, _ := service.h.Handler(r)
	h.ServeHTTP(&httpWriter{current: current, header: make(http.Header)}, r)
}

func (service *ServiceHttp) HandleError(current *CurrentContent, err error) {
	service.imp.HandleError(current, err)
}

func (service *ServiceHttp) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	nIndex := bytes.Index(data, []byte{'\r', '\n', '\r', '\n'})
	if nIndex <= 0 {
		return 0, 0, nil, nil
	}
	dataLen := nIndex + 4
	tp := textproto.NewReader(bufio.NewReader(bytes.NewReader(data[0:dataLen])))

	var line string
	if line, err = tp.ReadLine(); err != nil {
		return len(data), 0, nil, err
	}
	//"GET /foo HTTP/1.1"
	ss := strings.Split(line, " ")
	if len(ss) != 3 {
		return len(data), 0, nil, fmt.Errorf("not http")
	}
	if ss[2] != "HTTP/1.1" {
		return len(data), 0, nil, fmt.Errorf("not http/1.1 method:%s requestURI:%s requestURI:%s", ss[0], ss[1], ss[2])
	}

	mimeHeader, err := tp.ReadMIMEHeader()
	if err != nil {
		return len(data), 0, nil, err
	}

	cl := mimeHeader.Get("Content-Length")
	cl = textproto.TrimString(cl)
	if len(cl) > 0 { //fixed length
		n, err := strconv.ParseUint(cl, 10, 63)
		if err != nil {
			return len(data), 0, nil, fmt.Errorf("bad Content-Length %s", cl)
		}
		dataLen += int(n)
		if len(data) < dataLen {
			return 0, 0, nil, nil
		}
	} else {
		te := mimeHeader.Get("Transfer-Encoding")
		te = textproto.TrimString(te)
		if te == "chunked" {
			i := bytes.Index(data, []byte{'\r', '\n', '0', '\r', '\n'})
			if i <= 0 {
				return 0, 0, nil, nil
			}
			dataLen = i + 5
		}
	}

	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(data[0:dataLen])))
	if err != nil {
		return dataLen, 0, nil, err
	}
	return dataLen, 0, req, nil
}

func (service *ServiceHttp) HashProcessor(current *CurrentContent, msgID uint64, msg interface{}) (processorID int) {
	var req *http.Request
	if msg != nil {
		req = msg.(*http.Request)
	}
	return service.imp.HashProcessor(current, req)
}

type HttpHandler struct {
	mu    sync.RWMutex
	m     map[string]muxEntry
	es    []muxEntry // slice of entries sorted from longest to shortest.
	hosts bool       // whether any patterns contain hostnames
}

type muxEntry struct {
	h       http.Handler
	pattern string
}

func (h *HttpHandler) Handler(r *http.Request) (http.Handler, string) {
	host := r.Host
	if strings.Contains(host, ":") {
		h1, _, err := net.SplitHostPort(host)
		if err != nil {
			host = h1
		}
	}

	return h.handler(host, r.URL.Path)
}

// Handle registers the handler for the given pattern
// in the DefaultServeMux.
// The documentation for ServeMux explains how patterns are matched.
func (h *HttpHandler) Handle(pattern string, handler http.Handler) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if pattern == "" {
		panic("http: invalid pattern")
	}
	if handler == nil {
		panic("http: nil handler")
	}
	if _, exist := h.m[pattern]; exist {
		panic("http: multiple registrations for " + pattern)
	}

	if h.m == nil {
		h.m = make(map[string]muxEntry)
	}
	e := muxEntry{h: handler, pattern: pattern}
	h.m[pattern] = e
	if pattern[len(pattern)-1] == '/' {
		h.es = appendSorted(h.es, e)
	}

	if pattern[0] != '/' {
		h.hosts = true
	}
}

// HandleFunc registers the handler function for the given pattern.
func (h *HttpHandler) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	if handler == nil {
		panic("http: nil handler")
	}
	h.Handle(pattern, http.HandlerFunc(handler))
}

func appendSorted(es []muxEntry, e muxEntry) []muxEntry {
	n := len(es)
	i := sort.Search(n, func(i int) bool {
		return len(es[i].pattern) < len(e.pattern)
	})
	if i == n {
		return append(es, e)
	}
	// we now know that 'i' points at where we want to insert
	es = append(es, muxEntry{}) // try to grow the slice in place, any entry works.
	copy(es[i+1:], es[i:])      // Move shorter entries down
	es[i] = e
	return es
}

// Find a handler on a handler map given a path string.
// Most-specific (longest) pattern wins.
func (h *HttpHandler) match(path string) (h1 http.Handler, pattern string) {
	// Check for exact match first.
	v, ok := h.m[path]
	if ok {
		return v.h, v.pattern
	}

	// Check for longest valid match.  mux.es contains all patterns
	// that end in / sorted from longest to shortest.
	for _, e := range h.es {
		if strings.HasPrefix(path, e.pattern) {
			return e.h, e.pattern
		}
	}
	return nil, ""
}

// handler is the main implementation of Handler.
// The path is known to be in canonical form, except for CONNECT methods.
func (h *HttpHandler) handler(host, path string) (h1 http.Handler, pattern string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Host-specific pattern takes precedence over generic ones
	if h.hosts {
		h1, pattern = h.match(host + path)
	}
	if h1 == nil {
		h1, pattern = h.match(path)
	}
	if h1 == nil {
		h1, pattern = http.NotFoundHandler(), ""
	}
	return
}

// ServeHTTP dispatches the request to the handler whose
// pattern most closely matches the request URL.
func (h *HttpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.RequestURI == "*" {
		if r.ProtoAtLeast(1, 1) {
			w.Header().Set("Connection", "close")
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	h1, _ := h.Handler(r)
	h1.ServeHTTP(w, r)
}
