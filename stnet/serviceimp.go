package stnet

type FuncHandleMessage func(*Session, interface{})

type ServiceImp interface {
	Init() bool
	Loop()
	Destroy()
	RegisterMessage(s *Service) //s.RegisterMessage(msgid, FuncHandleMessage)

	Unmarshal(sess *Session, data []byte) (lenParsed int, msgID uint32, msg interface{}, err error) //must be rewrite
	//if return < 0, it only use main thread of the service.it should between 0-63
	HashHandleThread(sess *Session) int

	SessionOpen(sess *Session)
	SessionClose(sess *Session)

	HandleError(*Session, error)
}
