package sdp

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"

	. "github.com/gotask/gost/stnet"
)

var (
	MinMsgLen uint32 = 4
	MaxMsgLen uint32 = 1024 * 1024 * 100
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
func (service *ServiceSdp) HandleMessage(sess *Session, msgID uint32, msg interface{}) {
	//req := msg.(*ReqProto)
}
func (service *ServiceSdp) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int32, msg interface{}, err error) {
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
func (service *ServiceSdp) HashHandleThread(sess *Session) int {
	return -1
}
func (service *ServiceSdp) SessionOpen(sess *Session) {

}
func (service *ServiceSdp) SessionClose(sess *Session) {

}
func (service *ServiceSdp) HandleError(sess *Session, err error) {
	SysLog.Error(err.Error())
	sess.Close()
}

type ConnectSdp struct {
}

func (cs *ConnectSdp) Init() bool {
	return true
}
func (cs *ConnectSdp) Loop() {

}
func (cs *ConnectSdp) Destroy() {

}
func (cs *ConnectSdp) HandleMessage(sess *Session, msgID uint32, msg interface{}) {
	//rsp := msg.(*RspProto)
}
func (cs *ConnectSdp) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int32, msg interface{}, err error) {
	if len(data) < 4 {
		return 0, 0, nil, nil
	}
	msgLen := SdpLen(data)
	codeType := msgLen >> 24 & 0x01
	msgLen = msgLen & 0xFFFFFF
	if msgLen < MinMsgLen || msgLen > MaxMsgLen {
		return int(msgLen), 0, nil, fmt.Errorf("message length is wrong;len=%d;", msgLen)
	}
	if len(data) < int(msgLen) {
		return 0, 0, nil, nil
	}

	rsp := &RspProto{}
	if codeType > 0 {
		b := bytes.NewReader(data[4:msgLen])
		var out bytes.Buffer
		r, _ := zlib.NewReader(b)
		io.Copy(&out, r)

		err = Decode(rsp, out.Bytes())

	} else {
		err = Decode(rsp, data[4:msgLen])
	}
	if err != nil {
		return int(msgLen), 0, nil, err
	}

	return int(msgLen), 0, rsp, nil
}
func (service *ConnectSdp) HashHandleThread(sess *Session) int {
	return -1
}
func (service *ConnectSdp) SessionOpen(sess *Session) {

}
func (service *ConnectSdp) SessionClose(sess *Session) {

}
func (service *ConnectSdp) HandleError(sess *Session, err error) {
	SysLog.Error(err.Error())
	sess.Close()
}
