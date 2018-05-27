package sdp

import (
	"fmt"

	. "github.com/gotask/gost/stnet"
)

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
	//req := msg.(*RspProto)
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
