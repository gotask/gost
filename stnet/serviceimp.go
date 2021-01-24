package stnet

import (
	"fmt"
	"net/http"
)

type ServiceImp interface {
	Init() bool
	Loop()
	Destroy()
	HandleMessage(current *CurrentContent, msgID uint64, msg interface{})
	HandleError(*CurrentContent, error)
	SessionOpen(sess *Session)
	SessionClose(sess *Session)
	HeartBeatTimeOut(sess *Session)

	//protocol parsed
	//lenParsed is the length readed from 'data'.
	//msgID and msg are messages parsed from data.
	//when lenParsed <= 0 or msgID < 0,msg and err will be ignored.
	Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error)
	//sess msgID msg are returned by func of Unmarshal
	//processorID is the thread who process this msg;if processorID < 0, it only use main thread of the service.it should between 0-ProcessorThreadsNum.
	HashProcessor(sess *Session, msgID uint64, msg interface{}) (processorID int)
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
