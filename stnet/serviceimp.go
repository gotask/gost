package stnet

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
