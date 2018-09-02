package stnet

import (
	"errors"

	"net"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

type CMDType int

const (
	Data CMDType = 0 + iota
	Open
	Close
	HeartBeat
	System
)

var (
	ErrSocketClosed   = errors.New("socket closed")
	ErrSocketIsOpen   = errors.New("socket is open")
	ErrSendOverTime   = errors.New("send message over time")
	ErrSendBuffIsFull = errors.New("send buffer is full")
	ErrMsgParseNil    = errors.New("MsgParse is nil")
)

type MsgParse interface {
	//*Session:session which recved message
	//CMDType:event type of msg
	//[]byte:recved data now;
	//int:length of recved data parsed;
	ParseMsg(sess *Session, data []byte) int

	SessionEvent(sess *Session, cmd CMDType)
}

//this will be called when session open
type FuncOnOpen func(*Session)

//this will be called when session closed
type FuncOnClose func(*Session)

//message recv buffer size
const (
	MsgBuffSize = 1024
	MinMsgSize  = 64

	//the length of send queue
	WriterListLen = 256
	RecvListLen   = 256
)

//session id
var GlobalSessionID uint64

type Session struct {
	parser    MsgParse
	id        uint64
	socket    net.Conn
	writer    chan []byte
	hander    chan []byte
	closer    chan int
	wg        *sync.WaitGroup
	onopen    FuncOnOpen
	onclose   FuncOnClose
	isclose   uint32
	heartbeat uint32

	UserData interface{}
}

func NewSession(con net.Conn, msgparse MsgParse, onopen FuncOnOpen, onclose FuncOnClose, heartbeat uint32) (*Session, error) {
	if msgparse == nil {
		return nil, ErrMsgParseNil
	}
	sess := &Session{
		id:        atomic.AddUint64(&GlobalSessionID, 1),
		socket:    con,
		writer:    make(chan []byte, WriterListLen), //It's OK to leave a Go channel open forever and never close it. When the channel is no longer used, it will be garbage collected.
		hander:    make(chan []byte, RecvListLen),
		closer:    make(chan int),
		wg:        &sync.WaitGroup{},
		parser:    msgparse,
		onopen:    onopen,
		onclose:   onclose,
		heartbeat: heartbeat,
	}
	asyncDo(sess.dosend, sess.wg)
	asyncDo(sess.dohand, sess.wg)

	go sess.dorecv()
	return sess, nil
}

func newConnSession(msgparse MsgParse, onopen FuncOnOpen, onclose FuncOnClose) (*Session, error) {
	if msgparse == nil {
		return nil, ErrMsgParseNil
	}
	sess := &Session{
		id:        atomic.AddUint64(&GlobalSessionID, 1),
		writer:    make(chan []byte, WriterListLen), //It's OK to leave a Go channel open forever and never close it. When the channel is no longer used, it will be garbage collected.
		hander:    make(chan []byte, RecvListLen),
		wg:        &sync.WaitGroup{},
		parser:    msgparse,
		onopen:    onopen,
		onclose:   onclose,
		isclose:   1,
		heartbeat: 0,
	}
	return sess, nil
}

func (s *Session) handlePanic() {
	if err := recover(); err != nil {
		SysLog.Critical("panic error: %v", err)
		SysLog.Critical("panic stack: %s", string(debug.Stack()))
		//close socket
		s.socket.Close()
	}
}

func (s *Session) restart(con net.Conn) error {
	if !atomic.CompareAndSwapUint32(&s.isclose, 1, 0) {
		return ErrSocketIsOpen
	}
	s.closer = make(chan int)
	s.socket = con
	asyncDo(s.dosend, s.wg)
	asyncDo(s.dohand, s.wg)
	go s.dorecv()
	return nil
}

func (s *Session) GetID() uint64 {
	return s.id
}

func (s *Session) Send(data []byte) error {
	msg := bp.Alloc(len(data))
	copy(msg, data)

	to := time.NewTimer(100 * time.Millisecond)
	select {
	case <-s.closer:
		to.Stop()
		return ErrSocketClosed
	case s.writer <- msg:
		to.Stop()
		return nil
	case <-to.C:
		to.Stop()
		SysLog.Error("session sending queue is full and the message is droped;sessionid=%d", s.id)
		return ErrSendOverTime
	}
}

func (s *Session) AsyncSend(data []byte) error {
	msg := bp.Alloc(len(data))
	copy(msg, data)

	select {
	case <-s.closer:
		return ErrSocketClosed
	case s.writer <- msg:
		return nil
	default:
		SysLog.Error("session sending queue is full and the message is droped;sessionid=%d", s.id)
		return ErrSendBuffIsFull
	}

}

func (s *Session) Close() {
	if s.IsClose() {
		return
	}
	s.socket.Close()
}

func (s *Session) IsClose() bool {
	return atomic.LoadUint32(&s.isclose) > 0
}

func (s *Session) dosend() {
	for {
		select {
		case <-s.closer:
			return
		case buf := <-s.writer:
			if _, err := s.socket.Write(buf); err != nil {
				s.socket.Close()
				return
			}
			bp.Free(buf)
		}
	}
}

func (s *Session) dorecv() {
	defer func() {
		if err := recover(); err != nil {
			SysLog.Critical("panic error: %v", err)
			SysLog.Critical("panic stack: %s", string(debug.Stack()))
		}
		//close socket
		s.socket.Close()
		close(s.closer)
		s.wg.Wait()
		s.onclose(s)
		atomic.CompareAndSwapUint32(&s.isclose, 0, 1)
	}()

	if s.onopen != nil {
		s.onopen(s)
	}
	s.parser.SessionEvent(s, Open)

	msgbuf := bp.Alloc(MsgBuffSize)
	for {
		n, err := s.socket.Read(msgbuf)
		if err != nil {
			s.parser.SessionEvent(s, Close)
			return
		}
		s.hander <- msgbuf[0:n]

		bufLen := len(msgbuf)
		if MinMsgSize < bufLen && n*2 < bufLen {
			msgbuf = bp.Alloc(bufLen / 2)
		} else if n == bufLen {
			msgbuf = bp.Alloc(bufLen * 2)
		} else {
			msgbuf = bp.Alloc(bufLen)
		}
	}
}

func (s *Session) dohand() {
	defer s.handlePanic()

	wt := time.Second * time.Duration(s.heartbeat)
	ht := time.NewTimer(wt)
	if s.heartbeat == 0 {
		ht.Stop()
	}
	var tempBuf []byte
	for {
		if s.heartbeat > 0 {
			ht.Reset(wt)
		}
		select {
		case <-s.closer:
			return
		case <-ht.C:
			s.parser.SessionEvent(s, HeartBeat)
		case buf := <-s.hander:
			if tempBuf != nil {
				buf = append(tempBuf, buf...)
			}
		anthorMsg:
			parseLen := s.parser.ParseMsg(s, buf)
			if parseLen >= len(buf) {
				tempBuf = nil
				bp.Free(buf)
			} else if parseLen > 0 {
				buf = buf[parseLen:]
				goto anthorMsg
			} else if parseLen == 0 {
				tempBuf = buf
			}
		}
	}
	ht.Stop()
}

func asyncDo(fn func(), wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		fn()
		wg.Done()
	}()
}
