package stnet

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
)

//ServiceImpBase
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
}
func (service *ServiceBase) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	return len(data), -1, nil, nil
}
func (service *ServiceBase) HashProcessor(sess *Session, msgID uint64, msg interface{}) (processorID int) {
	return -1
}

//ServiceImpEcho
type ServiceEcho struct {
	ServiceBase
}

func (service *ServiceEcho) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	sess.Send(data)
	return len(data), -1, nil, nil
}

func (service *ServiceEcho) HashProcessor(sess *Session, msgID uint64, msg interface{}) (processorID int) {
	return int(sess.GetID() % uint64(ProcessorThreadsNum))
}

//ServiceHttp
type ServiceHttp struct {
	ServiceBase
	imp HttpService
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
	ls := bytes.Index(data[0:dataLen], []byte("Content-Length: "))
	if ls >= 0 { //POST
		nd := data[ls+16:]
		end := bytes.IndexByte(nd, '\r')
		if end < 0 {
			return len(data), 0, nil, fmt.Errorf("error http header formate.")
		}
		leng := nd[:end]
		l, e := strconv.Atoi(string(leng))
		if e != nil {
			return len(data), 0, nil, fmt.Errorf("error http header formate.")
		}
		dataLen += l
		if len(data) < dataLen {
			return 0, 0, nil, nil
		}
	}
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(data[0:dataLen])))
	if err != nil {
		return dataLen, 0, nil, err
	}
	return dataLen, 0, req, nil
}
func (service *ServiceHttp) HashProcessor(sess *Session, msgID uint64, msg interface{}) (processorID int) {
	req := msg.(*http.Request)
	return service.imp.HashProcessor(sess, req)
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

//ServiceJson
type ServiceJson struct {
	ServiceBase
	imp JsonService
}

func (service *ServiceJson) Loop() {
	service.imp.Loop()
}
func (service *ServiceJson) HandleMessage(current *CurrentContent, msgID uint64, msg interface{}) {
	service.imp.Handle(current, msg.(*JsonMsg), nil)
}
func (service *ServiceJson) HandleError(current *CurrentContent, err error) {
	service.imp.Handle(current, nil, err)
}
func (service *ServiceJson) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	if len(data) < 4 {
		return 0, 0, nil, nil
	}
	msgLen := binary.BigEndian.Uint32(data)
	if msgLen < 4 || msgLen > 1024*1024*100 {
		return int(msgLen), 0, nil, fmt.Errorf("message length is invalid: %d", msgLen)
	}
	if len(data) < int(msgLen) {
		return 0, 0, nil, nil
	}

	m := &JsonMsg{}
	e := json.Unmarshal(data[4:msgLen], m)
	if e != nil {
		return int(msgLen), 0, nil, e
	}

	return int(msgLen), 0, m, nil
}
func (service *ServiceJson) HashProcessor(sess *Session, msgID uint64, msg interface{}) (processorID int) {
	return service.imp.HashProcessor(sess, msg.(*JsonMsg))
}

type ServiceProxyS struct {
	ServiceBase
	remote   *Service
	remoteip []string
	weight   []int
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
	if sess.UserData == nil {
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
	sess.UserData.(*Connect).Send(data)
	return len(data), -1, nil, nil
}
func (service *ServiceProxyS) HashProcessor(sess *Session, msgID uint64, msg interface{}) (processorID int) {
	return int(sess.GetID() % uint64(ProcessorThreadsNum))
}

type ServiceProxyC struct {
	ServiceBase
}

func (service *ServiceProxyC) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	sess.UserData.(*Session).Send(data)
	return len(data), -1, nil, nil
}
func (service *ServiceProxyC) HashProcessor(sess *Session, msgID uint64, msg interface{}) (processorID int) {
	return int(sess.GetID() % uint64(ProcessorThreadsNum))
}
