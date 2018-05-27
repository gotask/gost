package stnet

type FuncHandleMessage func(*Session, interface{})

type ServiceImp interface {
	Init() bool
	Loop()
	Destroy()
	RegisterSMessage(s *Service) //s.RegisterMessage(msgid, FuncHandleMessage)

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
	RegisterCMessage(c *Connect) //c.RegisterMessage(msgid, FuncHandleMessage)

	Unmarshal(sess *Session, data []byte) (lenParsed int, msgID uint32, msg interface{}, err error) //must be rewrite

	Connected(sess *Session)
	DisConnected(sess *Session)

	HandleError(*Session, error)
}
