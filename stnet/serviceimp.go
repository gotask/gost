package stnet

type ServiceImp interface {
	Init() bool
	Loop()
	Destroy()
	HandleMessage(sess *Session, msgID uint32, msg interface{})
	HandleError(*Session, error)
	SessionOpen(sess *Session)
	SessionClose(sess *Session)
	HeartBeatTimeOut(sess *Session)

	//protocol parsed
	//lenParsed is the length readed from 'data'.
	//processorID is the thread who process this msg;if processorID < 0, it only use main thread of the service.it should between 0-127.
	//msgID and msg are messages parsed from data.
	//when lenParsed <= 0 or msgID < 0,msg and err will be ignored.
	Unmarshal(sess *Session, data []byte) (lenParsed, processorID int, msgID int32, msg interface{}, err error)
}
