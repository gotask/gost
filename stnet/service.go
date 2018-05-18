package stnet

import (
	"fmt"
)

func newService(name, address string, imp ServiceImp) (*Service, error) {
	if imp == nil {
		return nil, fmt.Errorf("ServiceImp should not be nil")
	}
	svr := &Service{name, nil, imp, make(chan sessionMessage, 1024), make(map[uint32]FuncHandleMessage)}
	lis, err := NewListener(address, svr)
	if err != nil {
		return nil, err
	}
	svr.listen = lis
	return svr, nil
}

func (service *Service) RegisterMessage(msgID uint32, handler FuncHandleMessage) {
	if handler == nil {
		return
	}
	service.messageHandlers[msgID] = handler
}

type NullService struct {
	Name string
	imp  NullServiceImp
}

type Service struct {
	Name            string
	listen          *Listener
	imp             ServiceImp
	messageQ        chan sessionMessage
	messageHandlers map[uint32]FuncHandleMessage
}

type sessionMessage struct {
	Sess   *Session
	DtType CMDType
	MsgID  uint32
	Msg    interface{}
	Err    error
}

func (service *Service) loop() {
	for i := 0; i < 100; i++ {
		select {
		case msg := <-service.messageQ:
			if msg.Err != nil {
				service.imp.HandleError(msg.Sess, msg.Err)
			} else if msg.DtType == Open {
				service.imp.SessionOpen(msg.Sess)
			} else if msg.DtType == Close {
				service.imp.SessionClose(msg.Sess)
			} else if msg.DtType == Data {
				if handler, ok := service.messageHandlers[msg.MsgID]; ok {
					handler(msg.Sess, msg.Msg)
				} else {
					service.imp.HandleError(msg.Sess, fmt.Errorf("message handler not find"))
				}
			}
		default:
			break
		}
	}
}
func (service *Service) destroy() {
	service.listen.Close()
}
func (service *Service) ParseMsg(sess *Session, data []byte) int {
	lenParsed, msgid, msg, e := service.imp.Unmarshal(sess, data)
	service.messageQ <- sessionMessage{sess, Data, msgid, msg, e}
	return lenParsed
}
func (service *Service) SessionEvent(sess *Session, cmd CMDType) {
	service.messageQ <- sessionMessage{sess, cmd, 0, nil, nil}
}

func newConnect(name, address string, reconnectmsec int, imp ConnectImp) (*Connect, error) {
	if imp == nil {
		return nil, fmt.Errorf("ServiceImp should not be nil")
	}
	conn := &Connect{nil, name, imp, make(chan sessionMessage, 1024), make(map[uint32]FuncHandleMessage)}
	ct, err := NewConnector(address, reconnectmsec, conn, nil)
	if err != nil {
		return nil, err
	}
	conn.Connector = ct
	return conn, nil
}

func newConnectNoStart(name, address string, reconnectmsec int, imp ConnectImp) (*Connect, error) {
	if imp == nil {
		return nil, fmt.Errorf("ServiceImp should not be nil")
	}
	conn := &Connect{nil, name, imp, make(chan sessionMessage, 1024), make(map[uint32]FuncHandleMessage)}
	ct, err := NewConnectorNoStart(address, reconnectmsec, conn, nil)
	if err != nil {
		return nil, err
	}
	conn.Connector = ct
	return conn, nil
}

func (ct *Connect) RegisterMessage(msgID uint32, handler FuncHandleMessage) {
	if handler == nil {
		return
	}
	ct.messageHandlers[msgID] = handler
}

type Connect struct {
	*Connector
	Name            string
	imp             ConnectImp
	messageQ        chan sessionMessage
	messageHandlers map[uint32]FuncHandleMessage
}

func (ct *Connect) loop() {
	for i := 0; i < 100; i++ {
		select {
		case msg := <-ct.messageQ:
			if msg.Err != nil {
				ct.imp.HandleError(msg.Sess, msg.Err)
			} else if msg.DtType == Open {
				ct.imp.Connected(msg.Sess)
			} else if msg.DtType == Close {
				ct.imp.DisConnected(msg.Sess)
			} else if handler, ok := ct.messageHandlers[msg.MsgID]; ok {
				handler(msg.Sess, msg.Msg)
			} else {
				ct.imp.HandleError(msg.Sess, fmt.Errorf("message handler not find"))
			}
		default:
			break
		}
	}
}
func (ct *Connect) destroy() {
	ct.Connector.Close()
}
func (ct *Connect) ParseMsg(sess *Session, data []byte) int {
	lenParsed, msgid, msg, e := ct.imp.Unmarshal(sess, data)
	ct.messageQ <- sessionMessage{sess, Data, msgid, msg, e}
	return lenParsed
}
func (ct *Connect) SessionEvent(sess *Session, cmd CMDType) {
	ct.messageQ <- sessionMessage{sess, cmd, 0, nil, nil}
}
