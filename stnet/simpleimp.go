package stnet

import (
	"bufio"
	"bytes"
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
func (service *ServiceBase) HandleMessage(sess *Session, msgID uint32, msg interface{}) {

}
func (service *ServiceBase) SessionOpen(sess *Session) {

}
func (service *ServiceBase) SessionClose(sess *Session) {

}
func (service *ServiceBase) HeartBeatTimeOut(sess *Session) {

}
func (service *ServiceBase) HandleError(sess *Session, err error) {
	SysLog.Error(err.Error())
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
}

func (service *ServiceHttp) RspOk(sess *Session) {
	sRspPayload := "HTTP/1.1 200 OK\r\nContent-Length:0\r\n\r\n"
	sess.Send([]byte(sRspPayload))
}
func (service *ServiceHttp) Rsp404(sess *Session) {
	sRspPayload := "HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\n\r\n"
	sess.Send([]byte(sRspPayload))
}
func (service *ServiceHttp) RspString(sess *Session, rsp string) {
	sRspPayload := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Length:%d\r\n\r\n%s", len(rsp), rsp)
	sess.Send([]byte(sRspPayload))
}
func (service *ServiceHttp) HandleMessage(sess *Session, msgID uint32, msg interface{}) {
	//req:=msg.(*http.Request)
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
	return -1
}
