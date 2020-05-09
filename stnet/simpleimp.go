package stnet

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
func (service *ServiceBase) HandleMessage(current *CurrentContent, msgID uint32, msg interface{}) {

}
func (service *ServiceBase) SessionOpen(sess *Session) {

}
func (service *ServiceBase) SessionClose(sess *Session) {

}
func (service *ServiceBase) HeartBeatTimeOut(sess *Session) {

}
func (service *ServiceBase) HandleError(current *CurrentContent, err error) {
	SysLog.Error(err.Error())
}
func (service *ServiceBase) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int32, msg interface{}, err error) {
	return len(data), -1, nil, nil
}
func (service *ServiceBase) HashProcessor(sess *Session, msgID int32, msg interface{}) (processorID int) {
	return -1
}

//ServiceImpEcho
type ServiceEcho struct {
	ServiceBase
}

func (service *ServiceEcho) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int32, msg interface{}, err error) {
	sess.Send(data)
	return len(data), -1, nil, nil
}

func (service *ServiceEcho) HashProcessor(sess *Session, msgID int32, msg interface{}) (processorID int) {
	return int(sess.GetID() % uint64(ProcessorThreadsNum))
}

//ServiceHttp
type ServiceHttp struct {
	ServiceBase
	imp HttpService
}

func (service *ServiceHttp) HandleMessage(current *CurrentContent, msgID uint32, msg interface{}) {
	req := msg.(*http.Request)
	service.imp.Handle(current, req, nil)
}
func (service *ServiceHttp) HandleError(current *CurrentContent, err error) {
	service.imp.Handle(current, nil, err)
}
func (service *ServiceHttp) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int32, msg interface{}, err error) {
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(data)))
	if err == io.EOF { //read partly
		return 0, 0, nil, nil
	} else if err != nil {
		return len(data), 0, nil, err
	}
	return len(data), 0, req, nil
}
func (service *ServiceHttp) HashProcessor(sess *Session, msgID int32, msg interface{}) (processorID int) {
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
func (service *ServiceJson) HandleMessage(current *CurrentContent, msgID uint32, msg interface{}) {
	service.imp.Handle(current, msg.(*JsonMsg), nil)
}
func (service *ServiceJson) HandleError(current *CurrentContent, err error) {
	service.imp.Handle(current, nil, err)
}
func (service *ServiceJson) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int32, msg interface{}, err error) {
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
func (service *ServiceJson) HashProcessor(sess *Session, msgID int32, msg interface{}) (processorID int) {
	return service.imp.HashProcessor(sess, msg.(*JsonMsg))
}
