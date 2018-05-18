package stnet

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
)

type FuncHandleMessage func(*Session, interface{})

type ServiceImp interface {
	Init() bool
	Loop()
	Destroy()
	RegisterSMessage(*Service)

	Unmarshal(sess *Session, data []byte) (lenParsed int, msgID uint32, msg interface{}, err error) //must be rewrite

	SessionOpen(sess *Session)
	SessionClose(sess *Session)

	HandleError(*Session, error)
}

type NullServiceImp interface {
	Init() bool
	Loop()
	Destroy()
}

type ConnectImp interface {
	RegisterCMessage(*Connect)

	Unmarshal(sess *Session, data []byte) (lenParsed int, msgID uint32, msg interface{}, err error) //must be rewrite

	Connected(sess *Session)
	DisConnected(sess *Session)

	HandleError(*Session, error)
}

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
func (service *ServiceEcho) RegisterSMessage(*Service) {

}
func (service *ServiceEcho) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID uint32, msg interface{}, err error) {
	sess.Send(data)
	return len(data), 0, nil, nil
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
func (service *ServiceHttp) RegisterSMessage(s *Service) {
	s.RegisterMessage(0, service.HandleHttpReq)
}
func (service *ServiceHttp) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID uint32, msg interface{}, err error) {
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
	fmt.Println(err.Error())
}

//ServiceImpSdp
type ReqProto struct {
	ReqCmdId  uint32 `tag:"0" require:"true"`
	ReqCmdSeq uint32 `tag:"1"`
	ReqData   string `tag:"5"`
}
type RspProto struct {
	RspCmdId  uint32 `tag:"0" require:"true"`
	RspCmdSeq uint32 `tag:"1"`
	PushSeqId uint32 `tag:"2"`
	RspCode   int32  `tag:"5"`
	RspData   string `tag:"6"`
}
type ServiceSdp struct {
}

func (service *ServiceSdp) Init() bool {
	return true
}
func (service *ServiceSdp) Loop() {

}
func (service *ServiceSdp) Destroy() {

}
func (service *ServiceSdp) HandleReqProto(s *Session, msg interface{}) {
	//req := msg.(*ReqProto)
}
func (service *ServiceSdp) RegisterSMessage(s *Service) {
	s.RegisterMessage(0, service.HandleReqProto)
}
func (service *ServiceSdp) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID uint32, msg interface{}, err error) {
	if len(data) < 4 {
		return 0, 0, nil, nil
	}
	msgLen := SdpLen(data)
	if len(data) < int(msgLen) {
		return 0, 0, nil, nil
	}
	req := &ReqProto{}
	e := Decode(req, data[4:msgLen])
	if e != nil {
		return int(msgLen), 0, nil, e
	}
	return int(msgLen), 0, req, nil
}
func (service *ServiceSdp) SessionOpen(sess *Session) {

}
func (service *ServiceSdp) SessionClose(sess *Session) {

}
func (service *ServiceSdp) HandleError(sess *Session, err error) {
	fmt.Println(err.Error())
}

type ConnectSdp struct {
}

func (cs *ConnectSdp) HandleRspProto(s *Session, msg interface{}) {
	//req := msg.(*ReqProto)
}
func (cs *ConnectSdp) RegisterCMessage(c *Connect) {
	c.RegisterMessage(0, cs.HandleRspProto)
}
func (cs *ConnectSdp) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID uint32, msg interface{}, err error) {
	if len(data) < 4 {
		return 0, 0, nil, nil
	}
	msgLen := SdpLen(data)
	if len(data) < int(msgLen) {
		return 0, 0, nil, nil
	}
	rsp := &RspProto{}
	e := Decode(rsp, data[4:msgLen])
	if e != nil {
		return int(msgLen), 0, nil, e
	}
	return int(msgLen), 0, rsp, nil
}

func (cs *ConnectSdp) Connected(sess *Session) {

}
func (cs *ConnectSdp) DisConnected(sess *Session) {

}
func (cs *ConnectSdp) HandleError(s *Session, err error) {
	fmt.Println(err.Error())
}
