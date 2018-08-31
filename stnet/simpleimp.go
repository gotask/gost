package stnet

import (
	"bufio"
	"bytes"
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

func (service *ServiceEcho) Unmarshal(sess *Session, data []byte) (lenParsed, processorID int, msgID int32, msg interface{}, err error) {
	sess.Send(data)
	return len(data), int(sess.GetID() % uint64(ProcessorThreadsNum)), -1, nil, nil
}

//ServiceHttp
type ServiceHttp struct {
	ServiceBase
}

func (service *ServiceHttp) HandleMessage(sess *Session, msgID uint32, msg interface{}) {
	//req:=msg.(*http.Request)
}
func (service *ServiceHttp) Unmarshal(sess *Session, data []byte) (lenParsed, processorID int, msgID int32, msg interface{}, err error) {
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(data)))
	if err != nil {
		return 0, 0, 0, nil, nil
	}
	return len(data), int(sess.GetID() % uint64(ProcessorThreadsNum)), 0, req, nil
}
