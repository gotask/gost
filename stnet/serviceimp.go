package stnet

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
)

type ServiceImp interface {
	Init() bool
	Loop()
	Destroy()
	HandleMessage(current *CurrentContent, msgID uint32, msg interface{})
	HandleError(*CurrentContent, error)
	SessionOpen(sess *Session)
	SessionClose(sess *Session)
	HeartBeatTimeOut(sess *Session)

	//protocol parsed
	//lenParsed is the length readed from 'data'.
	//msgID and msg are messages parsed from data.
	//when lenParsed <= 0 or msgID < 0,msg and err will be ignored.
	Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int32, msg interface{}, err error)
	//sess msgID msg are returned by func of Unmarshal
	//processorID is the thread who process this msg;if processorID < 0, it only use main thread of the service.it should between 0-ProcessorThreadsNum.
	HashProcessor(sess *Session, msgID int32, msg interface{}) (processorID int)
}

type LoopService interface {
	Init() bool
	Loop()
}

type HttpService interface {
	Handle(current *CurrentContent, req *http.Request, e error)
	HashProcessor(sess *Session, req *http.Request) (processorID int)
}

func RspOk(sess *Session) {
	sRspPayload := "HTTP/1.1 200 OK\r\nContent-Length:0\r\n\r\n"
	sess.Send([]byte(sRspPayload))
}
func Rsp404(sess *Session) {
	sRspPayload := "HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\n\r\n"
	sess.Send([]byte(sRspPayload))
}
func RspString(sess *Session, rsp string) {
	sRspPayload := fmt.Sprintf("HTTP/1.1 200 OK\r\nContent-Length:%d\r\n\r\n%s", len(rsp), rsp)
	sess.Send([]byte(sRspPayload))
}

type JsonMsg struct {
	Seq     int64  `json:"s"`
	RetCode int64  `json:"r"`
	CmdId   int64  `json:"i"`
	CmdJson string `json:"j"`
}

type JsonService interface {
	Loop()
	Handle(current *CurrentContent, msg *JsonMsg, e error)
	HashProcessor(sess *Session, msg *JsonMsg) (processorID int)
}

func JsonSendBuffer(seq, ret, cmdid int64, cmd interface{}) []byte {
	msg := JsonMsg{seq, ret, cmdid, ""}
	b, _ := json.Marshal(cmd)
	if len(b) > 0 {
		msg.CmdJson = string(b)
	}
	data, _ := json.Marshal(&msg)
	msglen := len(data) + 4
	buf := make([]byte, msglen, msglen)
	binary.BigEndian.PutUint32(buf, uint32(msglen))
	copy(buf[4:], data)
	return buf
}
