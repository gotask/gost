package stnet

type ServiceImp interface {
	Init() bool
	Loop()
	Destroy()
	HandleMessage(sess *Session, msgID uint32, msg interface{})

	//protocol parsed
	//lenParsed is the length readed from 'data';msgID and msg are messages parsed from data;
	//when lenParsed <= 0 or msgID < 0,msg and err will be ignored.
	Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int32, msg interface{}, err error)

	//if return < 0, it only use main thread of the service.it should between 0-63
	HashHandleThread(sess *Session) int

	SessionOpen(sess *Session)
	SessionClose(sess *Session)

	HandleError(*Session, error)
}
