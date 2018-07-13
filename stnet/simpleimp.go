package stnet

import (
	"bufio"
	"bytes"
	"fmt"
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
func (service *ServiceEcho) RegisterMessage(*Service) {

}
func (service *ServiceEcho) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID uint32, msg interface{}, err error) {
	sess.Send(data)
	return len(data), 0, nil, nil
}
func (service *ServiceEcho) HashHandleThread(sess *Session) int {
	return -1
}
func (service *ServiceEcho) SessionOpen(sess *Session) {

}
func (service *ServiceEcho) SessionClose(sess *Session) {

}
func (service *ServiceEcho) HandleError(sess *Session, err error) {
	fmt.Println(err.Error())
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
func (service *ServiceHttp) HandleHttpReq(s *Session, msg interface{}) {
	//req := msg.(*http.Request)
}
func (service *ServiceHttp) RegisterMessage(s *Service) {
	s.RegisterMessage(0, service.HandleHttpReq)
}
func (service *ServiceHttp) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID uint32, msg interface{}, err error) {
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(data)))
	if err != nil {
		return 0, 0, nil, nil
	}
	return len(data), 0, req, nil
}
func (service *ServiceHttp) HashHandleThread(sess *Session) int {
	return -1
}
func (service *ServiceHttp) SessionOpen(sess *Session) {

}
func (service *ServiceHttp) SessionClose(sess *Session) {

}
func (service *ServiceHttp) HandleError(sess *Session, err error) {
	fmt.Println(err.Error())
}
