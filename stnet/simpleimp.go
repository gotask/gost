package stnet

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
)

// ServiceImpBase
type ServiceBase struct {
}

func (service *ServiceBase) Init() bool {
	return true
}
func (service *ServiceBase) Loop() {

}
func (service *ServiceBase) Destroy() {

}
func (service *ServiceBase) HandleMessage(current *CurrentContent, msgID uint64, msg interface{}) {

}
func (service *ServiceBase) SessionOpen(sess *Session) {

}
func (service *ServiceBase) SessionClose(sess *Session) {

}
func (service *ServiceBase) HeartBeatTimeOut(sess *Session) {
	sess.Close()
}
func (service *ServiceBase) HandleError(current *CurrentContent, err error) {
	SysLog.Error(err.Error())
	current.Sess.Close()
}
func (service *ServiceBase) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	return len(data), -1, nil, nil
}
func (service *ServiceBase) HashProcessor(current *CurrentContent, msgID uint64, msg interface{}) (processorID int) {
	return -1
}

// ServiceImpEcho
type ServiceEcho struct {
	ServiceBase
}

func (service *ServiceEcho) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	sess.Send(data, sess.peer)
	return len(data), -1, nil, nil
}

func (service *ServiceEcho) HashProcessor(current *CurrentContent, msgID uint64, msg interface{}) (processorID int) {
	return int(current.Sess.GetID())
}

// ServiceHttp
type ServiceHttp struct {
	ServiceBase
	imp HttpService
}

func (service *ServiceHttp) Init() bool {
	return service.imp.Init()
}

func (service *ServiceHttp) Loop() {
	service.imp.Loop()
}

func (service *ServiceHttp) HandleMessage(current *CurrentContent, msgID uint64, msg interface{}) {
	req := msg.(*http.Request)
	service.imp.Handle(current, req, nil)
}
func (service *ServiceHttp) HandleError(current *CurrentContent, err error) {
	service.imp.Handle(current, nil, err)
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
			return len(data), 0, nil, fmt.Errorf("bad Content-Length %d", cl)
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

type ServiceLoop struct {
	ServiceBase
	imp LoopService
}

func (service *ServiceLoop) Init() bool {
	return service.imp.Init()
}

func (service *ServiceLoop) Loop() {
	service.imp.Loop()
}

type ServiceProxyS struct {
	ServiceBase
	remote   *Service
	remoteip []string
	weight   []int
}

func (service *ServiceProxyS) SessionOpen(sess *Session) {
	rip := service.remoteip[0]
	ln := len(service.weight)
	if ln > 1 {
		r := rand.Int() % service.weight[ln-1]
		for i := 0; i < ln; i++ {
			if r < service.weight[i] {
				rip = service.remoteip[i]
				break
			}
		}
	}
	sess.UserData = service.remote.NewConnect(rip, sess)
}
func (service *ServiceProxyS) SessionClose(sess *Session) {
	if sess.UserData != nil {
		sess.UserData.(*Connect).Close()
	}
}
func (service *ServiceProxyS) HeartBeatTimeOut(sess *Session) {
	sess.Close()
}
func (service *ServiceProxyS) HandleError(current *CurrentContent, err error) {
	current.Sess.Close()
}
func (service *ServiceProxyS) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	sess.UserData.(*Connect).Send(data)
	return len(data), -1, nil, nil
}
func (service *ServiceProxyS) HashProcessor(current *CurrentContent, msgID uint64, msg interface{}) (processorID int) {
	return int(current.Sess.GetID())
}

type ServiceProxyC struct {
	ServiceBase
}

func (service *ServiceProxyC) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	sess.UserData.(*Session).Send(data, sess.peer)
	return len(data), -1, nil, nil
}
func (service *ServiceProxyC) HashProcessor(current *CurrentContent, msgID uint64, msg interface{}) (processorID int) {
	return int(current.Sess.GetID())
}

// ServiceJson
type ServiceJson struct {
	ServiceBase
	imp JsonService
}

func (service *ServiceJson) Init() bool {
	return service.imp.Init()
}

func (service *ServiceJson) Loop() {
	service.imp.Loop()
}

func (service *ServiceJson) HandleMessage(current *CurrentContent, msgID uint64, msg interface{}) {
	var d []byte
	if msg != nil {
		d = msg.([]byte)
	}
	service.imp.Handle(current, JsonProto{msgID, d}, nil)
}
func (service *ServiceJson) HandleError(current *CurrentContent, err error) {
	service.imp.Handle(current, JsonProto{}, err)
}
func (service *ServiceJson) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	if len(data) < 4 {
		return 0, 0, nil, nil
	}
	msgLen := MsgLen(data)
	if msgLen < 4 || msgLen >= uint32(MaxMsgSize) {
		return len(data), 0, nil, fmt.Errorf("message length is invalid: %d", msgLen)
	}

	if len(data) < int(msgLen) {
		return 0, 0, nil, nil
	}
	cmd := JsonProto{}
	e := json.Unmarshal(data[4:msgLen], &cmd)
	if e != nil {
		return int(msgLen), 0, nil, e
	}
	return int(msgLen), int64(cmd.CmdId), cmd.CmdData, nil
}
func (service *ServiceJson) HashProcessor(current *CurrentContent, msgID uint64, msg interface{}) (processorID int) {
	var d []byte
	if msg != nil {
		d = msg.([]byte)
	}
	return service.imp.HashProcessor(current, JsonProto{msgID, d})
}

func SendJsonCmd(sess *Session, msgID uint64, msg []byte) error {
	cmd := JsonProto{msgID, msg}
	buf, e := EncodeProtocol(cmd, EncodeTyepJson)
	if e != nil {
		return e
	}
	return sess.Send(buf, nil)
}
