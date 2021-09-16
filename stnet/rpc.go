package stnet

import (
	"errors"
	"fmt"
	"net"
	"reflect"
	"sync"
	"time"
)

var (
	TimeOut int64 = 5 //sec
)

const (
	RpcErrNoRemoteFunc = -1
	RpcErrCallTimeout  = -2
	RpcErrFuncParamErr = -3
)

type ReqProto struct {
	ReqCmdId  uint32
	ReqCmdSeq uint32
	ReqData   []byte
	IsOneWay  bool
	FuncName  string
}

type RspProto struct {
	RspCmdId  uint32
	RspCmdSeq uint32
	PushSeqId uint32
	RspCode   int32
	RspData   []byte
	FuncName  string
}

type RpcService interface {
	Loop()
	HandleError(current *CurrentContent, err error)
	HandleReq(current *CurrentContent, msg *ReqProto)
	HandleRsp(current *CurrentContent, msg *RspProto)
	HashProcessor(current *CurrentContent) (processorID int)
}

type RpcFuncException func(rspCode int32)

type rpcRequest struct {
	req       ReqProto
	callback  interface{}
	exception RpcFuncException
	timeout   int64
	sess      *Session

	signal chan *RspProto
}

type ServiceRpc struct {
	ServiceBase
	imp     RpcService
	methods map[string]reflect.Method

	rpcRequests    map[uint32]*rpcRequest
	rpcReqSequence uint32
	rpcMutex       sync.Mutex
}

func NewServiceRpc(imp RpcService) *ServiceRpc {
	svr := &ServiceRpc{}
	svr.imp = imp
	svr.rpcRequests = make(map[uint32]*rpcRequest)
	svr.methods = make(map[string]reflect.Method)

	t := reflect.TypeOf(imp)
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		svr.methods[m.Name] = m
	}
	return svr
}

//rpc_call syncORasync remotesession udppeer remotefunction functionparams callback exception
func (service *ServiceRpc) rpc_call(issync bool, sess *Session, peer net.Addr, funcName string, params ...interface{}) error {
	var rpcReq rpcRequest
	rpcReq.timeout = time.Now().Unix() + TimeOut
	rpcReq.req.FuncName = funcName

	if len(params) < 2 {
		return errors.New("no callback function or exception function")
	}
	if params[len(params)-1] != nil {
		if ex, ok := params[len(params)-1].(func(int32)); ok {
			rpcReq.exception = ex
		} else {
			return errors.New("invalid exception function")
		}
	}
	if params[len(params)-2] != nil {
		param2Type := reflect.TypeOf(params[len(params)-2])
		if param2Type.Kind() != reflect.Func {
			return errors.New("invalid callback function")
		}
	}
	rpcReq.callback = params[len(params)-2]

	params = params[0 : len(params)-2]
	var err error
	spb := Spb{}
	for i, v := range params {
		err = rpcMarshal(&spb, uint32(i+1), v)
		if err != nil {
			return fmt.Errorf("wrong params in RpcCall:%s(%d) %s", funcName, i, err.Error())
		}
	}
	rpcReq.req.ReqData = spb.buf
	if rpcReq.callback == nil && rpcReq.exception == nil {
		rpcReq.req.IsOneWay = true
	}

	service.rpcMutex.Lock()
	service.rpcReqSequence++
	rpcReq.req.ReqCmdSeq = service.rpcReqSequence
	rpcReq.sess = sess
	if issync {
		rpcReq.signal = make(chan *RspProto, 1)
	}
	service.rpcRequests[rpcReq.req.ReqCmdSeq] = &rpcReq
	service.rpcMutex.Unlock()

	err = service.sendRpcReq(sess, peer, rpcReq.req)
	if err != nil {
		return err
	}

	if !issync {
		return nil
	}

	to := time.NewTimer(time.Duration(TimeOut) * time.Second)
	select {
	case rsp := <-rpcReq.signal:
		service.handleRpcRsp(rsp)
	case <-to.C:
		service.rpcMutex.Lock()
		delete(service.rpcRequests, rpcReq.req.ReqCmdSeq)
		service.rpcMutex.Unlock()

		if rpcReq.exception != nil {
			rpcReq.exception(RpcErrCallTimeout)
		}
	}
	to.Stop()

	return nil
}

//RpcCall remotesession remotefunction(string) functionparams callback(could nil) exception(could nil)
func (service *ServiceRpc) RpcCall(sess *Session, funcName string, params ...interface{}) error {
	return service.rpc_call(false, sess, nil, funcName, params...)
}
func (service *ServiceRpc) RpcCall_Sync(sess *Session, funcName string, params ...interface{}) error {
	return service.rpc_call(true, sess, nil, funcName, params...)
}
func (service *ServiceRpc) UdpRpcCall(sess *Session, peer net.Addr, funcName string, params ...interface{}) error {
	return service.rpc_call(false, sess, peer, funcName, params...)
}
func (service *ServiceRpc) UdpRpcCall_Sync(sess *Session, peer net.Addr, funcName string, params ...interface{}) error {
	return service.rpc_call(true, sess, peer, funcName, params...)
}

func (service *ServiceRpc) Init() bool {
	return true
}

func (service *ServiceRpc) Loop() {
	now := time.Now().Unix()
	timeouts := make([]*rpcRequest, 0)
	service.rpcMutex.Lock()
	for k, v := range service.rpcRequests {
		if v.timeout < now {
			if v.exception != nil && v.signal != nil {
				timeouts = append(timeouts, v)
			}
			delete(service.rpcRequests, k)
		}
	}
	service.rpcMutex.Unlock()

	for _, v := range timeouts {
		v.exception(RpcErrCallTimeout)
	}

	service.imp.Loop()
}

func (service *ServiceRpc) handleRpcReq(current *CurrentContent, req *ReqProto) {
	var rsp RspProto
	rsp.RspCmdSeq = req.ReqCmdSeq
	rsp.FuncName = req.FuncName

	m, ok := service.methods[req.FuncName]
	if !ok {
		rsp.RspCode = RpcErrNoRemoteFunc
		service.sendRpcRsp(current, rsp)
		SysLog.Error("no rpc function: %s", req.FuncName)
		return
	}

	spb := Spb{[]byte(req.ReqData), 0}

	var e error
	funcT := m.Type
	funcVals := make([]reflect.Value, funcT.NumIn())
	funcVals[0] = reflect.ValueOf(service.imp)
	for i := 1; i < funcT.NumIn(); i++ {
		t := funcT.In(i)
		val := newValByType(t)
		e = rpcUnmarshal(&spb, uint32(i), val.Interface())
		if e != nil {
			rsp.RspCode = RpcErrFuncParamErr
			service.sendRpcRsp(current, rsp)
			SysLog.Error("function %s param unpack failed: %s", req.FuncName, e.Error())
			return
		}
		if t.Kind() == reflect.Ptr {
			funcVals[i] = val
		} else {
			funcVals[i] = val.Elem()
		}
	}
	funcV := m.Func
	returns := funcV.Call(funcVals)

	if req.IsOneWay {
		return
	}

	spbSend := Spb{}
	for i, v := range returns {
		e = rpcMarshal(&spbSend, uint32(i+1), v.Interface())
		if e != nil {
			rsp.RspCode = RpcErrFuncParamErr
			service.sendRpcRsp(current, rsp)
			SysLog.Error("function %s param pack failed: %s", req.FuncName, e.Error())
			return
		}
	}
	rsp.RspData = spbSend.buf
	service.sendRpcRsp(current, rsp)
}

func (service *ServiceRpc) handleRpcRsp(rsp *RspProto) {
	service.rpcMutex.Lock()
	v, ok := service.rpcRequests[rsp.RspCmdSeq]
	if !ok {
		service.rpcMutex.Unlock()
		SysLog.Error("recv rpc rsp but req not found, func: %s", rsp.FuncName)
		return
	}
	delete(service.rpcRequests, rsp.RspCmdSeq)
	service.rpcMutex.Unlock()

	if rsp.RspCode != 0 {
		if v.exception != nil {
			v.exception(rsp.RspCode)
		}
	} else {
		spb := Spb{rsp.RspData, 0}

		if v.callback != nil {
			var e error
			funcT := reflect.TypeOf(v.callback)
			funcVals := make([]reflect.Value, funcT.NumIn())
			for i := 0; i < funcT.NumIn(); i++ {
				t := funcT.In(i)
				val := newValByType(t)
				e = rpcUnmarshal(&spb, uint32(i+1), val.Interface())
				if e != nil {
					if v.exception != nil {
						v.exception(RpcErrFuncParamErr)
					}
					SysLog.Error("recv rpc rsp but unpack failed, func:%s,%s", rsp.FuncName, e.Error())
					return
				}
				if t.Kind() == reflect.Ptr {
					funcVals[i] = val
				} else {
					funcVals[i] = val.Elem()
				}
			}
			funcV := reflect.ValueOf(v.callback)
			funcV.Call(funcVals)
		}
	}
}

func (service *ServiceRpc) HandleMessage(current *CurrentContent, msgID uint64, msg interface{}) {
	if msgID == 0 {
		service.imp.HandleReq(current, msg.(*ReqProto))
	} else if msgID == 1 {
		service.imp.HandleRsp(current, msg.(*RspProto))
	} else if msgID == 2 { //rpc req
		service.handleRpcReq(current, msg.(*ReqProto))
	} else if msgID == 3 { //rpc rsp
		service.handleRpcRsp(msg.(*RspProto))
	} else {
		SysLog.Error("invalid msgid %d", msgID)
	}
}

func (service *ServiceRpc) HandleError(current *CurrentContent, err error) {
	service.imp.HandleError(current, err)
}

func spbLen(b []byte) uint32 {
	return uint32(b[3]) | uint32(b[2])<<8 | uint32(b[1])<<16 //| uint32(b[0])<<24
}

func (service *ServiceRpc) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	if len(data) < 4 {
		return 0, 0, nil, nil
	}
	msgLen := spbLen(data)
	if msgLen < 4 || msgLen >= 1024*1024*16 {
		return int(msgLen), 0, nil, fmt.Errorf("message length is invalid: %d", msgLen)
	}

	if len(data) < int(msgLen) {
		return 0, 0, nil, nil
	}

	flag := data[0]
	if flag&0x1 == 0 { //req
		req := &ReqProto{}
		e := Unmarshal(data[4:msgLen], req, 0)
		if e != nil {
			return int(msgLen), 0, nil, e
		}

		if flag&0x2 == 0 {
			return int(msgLen), 0, req, nil
		} else { //rpc
			return int(msgLen), 2, req, nil
		}
	} else { //rsp
		rsp := &RspProto{}
		e := Unmarshal(data[4:msgLen], rsp, 0)
		if e != nil {
			return int(msgLen), 0, nil, e
		}

		if flag&0x2 == 0 {
			return int(msgLen), 1, rsp, nil
		} else { //rpc
			service.rpcMutex.Lock()
			v, ok := service.rpcRequests[rsp.RspCmdSeq]
			if ok && v.signal != nil { //sync call
				v.signal <- rsp
				service.rpcMutex.Unlock()
				return int(msgLen), -1, nil, nil
			}
			service.rpcMutex.Unlock()
			//async call
			return int(msgLen), 3, rsp, nil
		}
	}
}

func (service *ServiceRpc) HashProcessor(current *CurrentContent, msgID uint64, msg interface{}) (processorID int) {
	return service.imp.HashProcessor(current)
}

func encodeProtocol(msg interface{}, encode int) ([]byte, error) {
	data, e := Marshal(msg, encode)
	if e != nil {
		return nil, e
	}
	msglen := len(data) + 4
	spbMsg := Spb{}
	spbMsg.packByte(byte(msglen >> 24))
	spbMsg.packByte(byte(msglen >> 16))
	spbMsg.packByte(byte(msglen >> 8))
	spbMsg.packByte(byte(msglen))
	spbMsg.packData(data)
	flag := spbMsg.buf[0]
	if flag > 0 {
		return nil, fmt.Errorf("msg is too long: %d,max size is 16k", msglen)
	}
	return spbMsg.buf, nil
}

func (service *ServiceRpc) SendUdpReq(sess *Session, peer net.Addr, req ReqProto) error {
	buf, e := encodeProtocol(&req, 0)
	if e != nil {
		return e
	}
	return sess.Send(buf, peer)
}

func (service *ServiceRpc) SendUdpRsp(sess *Session, peer net.Addr, rsp RspProto) error {
	buf, e := encodeProtocol(&rsp, 0)
	if e != nil {
		return e
	}
	buf[0] |= 0x1
	return sess.Send(buf, peer)
}

func (service *ServiceRpc) SendReq(sess *Session, req ReqProto) error {
	return service.SendUdpReq(sess, nil, req)
}

func (service *ServiceRpc) SendRsp(sess *Session, rsp RspProto) error {
	return service.SendUdpRsp(sess, nil, rsp)
}

func (service *ServiceRpc) sendRpcReq(sess *Session, peer net.Addr, req ReqProto) error {
	buf, e := encodeProtocol(&req, 0)
	if e != nil {
		return e
	}
	buf[0] |= 0x2
	return sess.Send(buf, peer)
}

func (service *ServiceRpc) sendRpcRsp(current *CurrentContent, rsp RspProto) error {
	buf, e := encodeProtocol(&rsp, 0)
	if e != nil {
		return e
	}
	buf[0] |= 0x3
	return current.Sess.Send(buf, current.Peer)
}
