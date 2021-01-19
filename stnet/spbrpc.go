package stnet

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
)

var (
	TimeOut int64 = 5 //sec
)

const (
	SpbRpcErrNoRemoteFunc = 1
	SpbRpcErrCallTimeout  = 2
	SpbRpcErrFuncParamErr = 3
)

type SpbReqProto struct {
	ReqCmdId  uint32 `tag:"0" require:"true"`
	ReqCmdSeq uint32 `tag:"1"`
	ReqData   []byte `tag:"2"`
	IsOneWay  bool   `tag:"3"`
	FuncName  string `tag:"4"`
}

type SpbRspProto struct {
	RspCmdId  uint32 `tag:"0" require:"true"`
	RspCmdSeq uint32 `tag:"1"`
	PushSeqId uint32 `tag:"2"`
	RspCode   int32  `tag:"3"`
	RspData   []byte `tag:"4"`
	FuncName  string `tag:"5"`
}

type SpbRpcService interface {
	Loop()
	HandleError(current *CurrentContent, err error)
	HandleReq(current *CurrentContent, msg *SpbReqProto)
	HandleRsp(current *CurrentContent, msg *SpbRspProto)
	HashProcessor(sess *Session) (processorID int)
}

type RpcFuncException func(rspCode int32)

type rpcRequest struct {
	req       SpbReqProto
	callback  interface{}
	exception RpcFuncException
	timeout   int64
}

type ServiceSpbRpc struct {
	ServiceBase
	imp SpbRpcService

	rpcRequests    map[uint32]*rpcRequest
	rpcReqSequence uint32
	rpcMutex       sync.Mutex
}

func NewServiceSpbRpc(imp SpbRpcService) *ServiceSpbRpc {
	svr := &ServiceSpbRpc{}
	svr.imp = imp
	return svr
}

func (service *ServiceSpbRpc) RpcCall(sess *Session, funcName string, params ...interface{}) error { //remotefunction functionparams callback exception
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
		for i := 0; i < param2Type.NumIn(); i++ {
			if param2Type.In(i).Kind() == reflect.Ptr {
				return errors.New("invalid callback function param, cannot be ptr")
			}
		}
	}
	rpcReq.callback = params[len(params)-2]

	params = params[0 : len(params)-2]
	spb := Spb{}
	for i, v := range params {
		err := spb.pack(uint32(i+1), v, true, true)
		if err != nil {
			return fmt.Errorf("wrong params in RpcCall:%s %s", funcName, err.Error())
		}
	}
	rpcReq.req.ReqData = spb.buf
	if rpcReq.callback == nil && rpcReq.exception == nil {
		rpcReq.req.IsOneWay = true
	}

	service.rpcMutex.Lock()
	service.rpcReqSequence++
	rpcReq.req.ReqCmdSeq = service.rpcReqSequence
	service.rpcRequests[rpcReq.req.ReqCmdSeq] = &rpcReq
	service.rpcMutex.Unlock()

	return service.sendRpcReq(sess, rpcReq.req)
}

func (service *ServiceSpbRpc) Init() bool {
	service.rpcRequests = make(map[uint32]*rpcRequest)
	return true
}

func (service *ServiceSpbRpc) Loop() {
	now := time.Now().Unix()
	service.rpcMutex.Lock()
	for k, v := range service.rpcRequests {
		if v.timeout < now {
			if v.exception != nil {
				v.exception(SpbRpcErrCallTimeout)
			}
			delete(service.rpcRequests, k)
		}
	}
	service.rpcMutex.Unlock()

	service.imp.Loop()
}

func (service *ServiceSpbRpc) handleRpcReq(current *CurrentContent, req *SpbReqProto) {
	var rsp SpbRspProto
	rsp.RspCmdSeq = req.ReqCmdSeq
	rsp.FuncName = req.FuncName

	m, ok := reflect.TypeOf(service.imp).MethodByName(req.FuncName)
	if !ok {
		rsp.RspCode = SpbRpcErrNoRemoteFunc
		service.sendRpcRsp(current.Sess, rsp)
		SysLog.Error("no rpc funciton:%s", req.FuncName)
		return
	}

	spb := Spb{[]byte(req.ReqData), 0}

	funcT := m.Type
	funcVals := make([]reflect.Value, funcT.NumIn())
	funcVals[0] = reflect.ValueOf(service.imp)
	for i := 1; i < funcT.NumIn(); i++ {
		t := funcT.In(i)
		val := newValByType(t)
		e := spb.unpack(val, true)
		if e != nil {
			rsp.RspCode = SpbRpcErrFuncParamErr
			service.sendRpcRsp(current.Sess, rsp)
			SysLog.Error("funciton %s param unpack failed: %s", req.FuncName, e.Error())
			return
		}
		funcVals[i] = val
	}
	funcV := m.Func
	returns := funcV.Call(funcVals)

	if req.IsOneWay {
		return
	}

	spbSend := Spb{}
	for i, v := range returns {
		e := spbSend.pack(uint32(i+1), v.Interface(), true, true)
		if e != nil {
			rsp.RspCode = SpbRpcErrFuncParamErr
			service.sendRpcRsp(current.Sess, rsp)
			SysLog.Error("funciton %s param pack failed: %s", req.FuncName, e.Error())
			return
		}
	}
	rsp.RspData = spbSend.buf
	service.sendRpcRsp(current.Sess, rsp)
}

func (service *ServiceSpbRpc) handleRpcRsp(rsp *SpbRspProto) {
	service.rpcMutex.Lock()
	v, ok := service.rpcRequests[rsp.RspCmdSeq]
	if !ok {
		service.rpcMutex.Unlock()
		SysLog.Error("recv rpc rsp but req not found, func:%d", rsp.FuncName)
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
			funcT := reflect.TypeOf(v.callback)
			funcVals := make([]reflect.Value, funcT.NumIn())
			for i := 0; i < funcT.NumIn(); i++ {
				t := funcT.In(i)
				val := newValByType(t)
				e := spb.unpack(val, true)
				if e != nil {
					if v.exception != nil {
						v.exception(SpbRpcErrFuncParamErr)
					}
					SysLog.Error("recv rpc rsp but unpack failed, func:%d,%s", rsp.FuncName, e.Error())
					return
				}
				funcVals[i] = val
			}
			funcV := reflect.ValueOf(v.callback)
			funcV.Call(funcVals)
		}
	}
}

func (service *ServiceSpbRpc) HandleMessage(current *CurrentContent, msgID uint64, msg interface{}) {
	if msgID == 0 {
		service.imp.HandleReq(current, msg.(*SpbReqProto))
	} else if msgID == 1 {
		service.imp.HandleRsp(current, msg.(*SpbRspProto))
	} else if msgID == 2 { //rpc req
		service.handleRpcReq(current, msg.(*SpbReqProto))
	} else if msgID == 3 { //rpc rsp
		service.handleRpcRsp(msg.(*SpbRspProto))
	} else {
		SysLog.Error("invalid msgid %d", msgID)
	}
}

func (service *ServiceSpbRpc) HandleError(current *CurrentContent, err error) {
	service.imp.HandleError(current, err)
}

func spbLen(b []byte) uint32 {
	return uint32(b[3]) | uint32(b[2])<<8 | uint32(b[1])<<16 //| uint32(b[0])<<24
}

func (service *ServiceSpbRpc) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	if len(data) < 4 {
		return 0, 0, nil, nil
	}
	msgLen := spbLen(data)
	if msgLen <= 4 || msgLen >= 1024*1024*16 {
		return int(msgLen), 0, nil, fmt.Errorf("message length is invalid: %d", msgLen)
	}
	if len(data) < int(msgLen) {
		return 0, 0, nil, nil
	}

	flag := data[0]
	if flag&0x1 == 0 { //req
		req := &SpbReqProto{}
		e := Decode(req, data[4:msgLen])
		if e != nil {
			return int(msgLen), 0, nil, e
		}

		if flag&0x2 == 0 {
			return int(msgLen), 0, req, nil
		} else { //rpc
			return int(msgLen), 2, req, nil
		}
	} else { //rsp
		rsp := &SpbRspProto{}
		e := Decode(rsp, data[4:msgLen])
		if e != nil {
			return int(msgLen), 0, nil, e
		}

		if flag&0x2 == 0 {
			return int(msgLen), 1, rsp, nil
		} else { //rpc
			return int(msgLen), 3, rsp, nil
		}
	}
}

func (service *ServiceSpbRpc) HashProcessor(sess *Session, msgID uint64, msg interface{}) (processorID int) {
	return service.imp.HashProcessor(sess)
}

func encodeSpbProtocol(msg interface{}) ([]byte, error) {
	data, e := Encode(msg)
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
		return nil, fmt.Errorf("msg is too long: %d,max size is 16k.", msglen)
	}
	return spbMsg.buf, nil
}

func (service *ServiceSpbRpc) SendReq(sess *Session, req SpbReqProto) error {
	buf, e := encodeSpbProtocol(req)
	if e != nil {
		return e
	}
	return sess.Send(buf)
}

func (service *ServiceSpbRpc) SendRsp(sess *Session, rsp SpbRspProto) error {
	buf, e := encodeSpbProtocol(rsp)
	if e != nil {
		return e
	}
	buf[0] |= 0x1
	return sess.Send(buf)
}

func (service *ServiceSpbRpc) sendRpcReq(sess *Session, req SpbReqProto) error {
	buf, e := encodeSpbProtocol(req)
	if e != nil {
		return e
	}
	buf[0] |= 0x2
	return sess.Send(buf)
}

func (service *ServiceSpbRpc) sendRpcRsp(sess *Session, rsp SpbRspProto) error {
	buf, e := encodeSpbProtocol(rsp)
	if e != nil {
		return e
	}
	buf[0] |= 0x3
	return sess.Send(buf)
}
