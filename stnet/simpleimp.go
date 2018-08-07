package stnet

import (
	"bufio"
	"bytes"
	"net/http"
)

//ServiceImpEcho
type ServiceEcho struct {
}

func (service *ServiceEcho) Init() bool {
	return true
}
func (service *ServiceEcho) Loop() {

}
func (service *ServiceEcho) Destroy() {

}
func (service *ServiceEcho) HandleMessage(sess *Session, msgID uint32, msg interface{}) {

}
func (service *ServiceEcho) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int32, msg interface{}, err error) {
	sess.Send(data)
	return len(data), -1, nil, nil
}
func (service *ServiceEcho) SessionOpen(sess *Session) {

}
func (service *ServiceEcho) SessionClose(sess *Session) {

}
func (service *ServiceEcho) HandleError(sess *Session, err error) {
	SysLog.Error(err.Error())
}

//ServiceHttp
type ServiceHttp struct {
}

func (service *ServiceHttp) Init() bool {
	return true
}
func (service *ServiceHttp) Loop() {

}
func (service *ServiceHttp) Destroy() {

}
func (service *ServiceHttp) HandleMessage(sess *Session, msgID uint32, msg interface{}) {
	//req:=msg.(*http.Request)
}
func (service *ServiceHttp) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int32, msg interface{}, err error) {
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(data)))
	if err != nil {
		return 0, 0, nil, nil
	}
	return len(data), 0, req, nil
}
func (service *ServiceHttp) SessionOpen(sess *Session) {

}
func (service *ServiceHttp) SessionClose(sess *Session) {

}
func (service *ServiceHttp) HandleError(sess *Session, err error) {
	SysLog.Error(err.Error())
}
