package stnet

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"
)

type SdpRpcReqProto struct {
	IsOneWay    bool
	RequestId   uint32
	ServiceName string
	FuncName    string
	ReqPayload  string
	Timeout     uint32
	Context     map[string]string
}

type SdpRpcRspProto struct {
	MfwRet     int32
	RequestId  uint32
	RspPayload string
	Context    map[string]string
}

type SdpRpcService interface {
	Loop()
	HandleError(current *CurrentContent, err error)
	HashProcessor(current *CurrentContent) (processorID int)
}

type sdpRpcRequest struct {
	req       SdpRpcReqProto
	callback  interface{}
	exception RpcFuncException
	timeout   int64
	sess      *Session

	signal chan *SdpRpcRspProto
}

type ServiceSdpRpc struct {
	ServiceBase
	imp     SdpRpcService
	methods map[string]reflect.Method

	rpcRequests    map[uint32]*sdpRpcRequest
	rpcReqSequence uint32
	rpcMutex       sync.Mutex

	isClient bool
}

func NewServiceSdpRpc(imp SdpRpcService) *ServiceSdpRpc {
	svr := &ServiceSdpRpc{}
	svr.imp = imp
	svr.rpcRequests = make(map[uint32]*sdpRpcRequest)
	svr.methods = make(map[string]reflect.Method)

	t := reflect.TypeOf(imp)
	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)
		svr.methods[m.Name] = m
	}
	return svr
}

//rpc_call syncORasync remotesession udppeer remoteservice remotefunction functionparams callback exception
//exception function: func(int32){}
func (service *ServiceSdpRpc) rpc_call(issync bool, sess *Session, serviceName, funcName string, params ...interface{}) error {
	var rpcReq sdpRpcRequest
	rpcReq.timeout = time.Now().Unix() + TimeOut
	rpcReq.req.Timeout = uint32(TimeOut)
	rpcReq.req.ServiceName = serviceName
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
		err = spb.pack(uint32(i+1), v, true, true)
		if err != nil {
			return fmt.Errorf("wrong params in RpcCall:%s(%d) %s", funcName, i, err.Error())
		}
	}
	rpcReq.req.ReqPayload = string(spb.buf)
	if rpcReq.callback == nil && rpcReq.exception == nil {
		rpcReq.req.IsOneWay = true
	}

	service.rpcMutex.Lock()
	service.rpcReqSequence++
	rpcReq.req.RequestId = service.rpcReqSequence
	rpcReq.sess = sess
	if issync {
		rpcReq.signal = make(chan *SdpRpcRspProto, 1)
	}
	service.rpcRequests[rpcReq.req.RequestId] = &rpcReq
	service.rpcMutex.Unlock()

	err = service.send(&CurrentContent{Sess: sess}, rpcReq.req)
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
		delete(service.rpcRequests, rpcReq.req.RequestId)
		service.rpcMutex.Unlock()

		if rpcReq.exception != nil {
			rpcReq.exception(RpcErrCallTimeout)
		}
	}
	to.Stop()

	return nil
}

//RpcCall remotesession remotefunction(string) functionparams callback(could nil) exception(could nil)
func (service *ServiceSdpRpc) RpcCall(sess *Session, serviceName, funcName string, params ...interface{}) error {
	return service.rpc_call(false, sess, serviceName, funcName, params...)
}
func (service *ServiceSdpRpc) RpcCall_Sync(sess *Session, serviceName, funcName string, params ...interface{}) error {
	return service.rpc_call(true, sess, serviceName, funcName, params...)
}

func (service *ServiceSdpRpc) Init() bool {
	return true
}

func (service *ServiceSdpRpc) Loop() {
	now := time.Now().Unix()
	timeouts := make([]*sdpRpcRequest, 0)
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

func (service *ServiceSdpRpc) handleRpcReq(current *CurrentContent, req *SdpRpcReqProto) {
	var rsp SdpRpcRspProto
	rsp.RequestId = req.RequestId
	rsp.Context = req.Context

	m, ok := service.methods[req.FuncName]
	if !ok {
		rsp.MfwRet = RpcErrNoRemoteFunc
		service.send(current, rsp)
		SysLog.Error("no rpc function: %s", req.FuncName)
		return
	}

	spb := Spb{[]byte(req.ReqPayload), 0}

	var e error
	funcT := m.Type
	funcVals := make([]reflect.Value, funcT.NumIn())
	funcVals[0] = reflect.ValueOf(service.imp)
	for i := 1; i < funcT.NumIn(); i++ {
		t := funcT.In(i)
		val := newValByType(t)
		e = spb.unpack(val, true)
		if e != nil {
			rsp.MfwRet = RpcErrFuncParamErr
			service.send(current, rsp)
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
	var r int32
	spbSend.pack(0, r, true, true)
	for i, v := range returns {
		e = spbSend.pack(uint32(len(funcVals)+i+1), v.Interface(), true, true)
		if e != nil {
			rsp.MfwRet = RpcErrFuncParamErr
			service.send(current, rsp)
			SysLog.Error("function %s param pack failed: %s", req.FuncName, e.Error())
			return
		}
	}
	rsp.RspPayload = string(spbSend.buf)
	service.send(current, rsp)
}

func (service *ServiceSdpRpc) handleRpcRsp(rsp *SdpRpcRspProto) {
	service.rpcMutex.Lock()
	v, ok := service.rpcRequests[rsp.RequestId]
	if !ok {
		service.rpcMutex.Unlock()
		SysLog.Error("recv rpc rsp but req not found, id: %d", rsp.RequestId)
		return
	}
	delete(service.rpcRequests, rsp.RequestId)
	service.rpcMutex.Unlock()

	if rsp.MfwRet != 0 {
		if v.exception != nil {
			v.exception(rsp.MfwRet)
		}
	} else {
		spb := Spb{[]byte(rsp.RspPayload), 0}

		if v.callback != nil {
			var r int32
			var e error
			var t reflect.Type
			var val reflect.Value

			funcT := reflect.TypeOf(v.callback)
			funcVals := make([]reflect.Value, funcT.NumIn())
			for i := 0; i < funcT.NumIn()+1; i++ {
				if i == 0 {
					e = spb.unpack(reflect.ValueOf(&r).Elem(), true)
				} else {
					t = funcT.In(i - 1)
					val = newValByType(t)
					e = spb.unpack(val, true)
				}
				if e != nil {
					if v.exception != nil {
						v.exception(RpcErrFuncParamErr)
					}
					SysLog.Error("recv rpc rsp but unpack failed, func:%s,%d,%s", v.req.FuncName, i, e.Error())
					return
				}
				if i > 0 {
					if t.Kind() == reflect.Ptr {
						funcVals[i-1] = val
					} else {
						funcVals[i-1] = val.Elem()
					}
				}
			}
			funcV := reflect.ValueOf(v.callback)
			funcV.Call(funcVals)
		}
	}
}

func (service *ServiceSdpRpc) HandleMessage(current *CurrentContent, msgID uint64, msg interface{}) {
	if msgID == 2 { //rpc req
		service.handleRpcReq(current, msg.(*SdpRpcReqProto))
	} else if msgID == 3 { //rpc rsp
		service.handleRpcRsp(msg.(*SdpRpcRspProto))
	} else {
		SysLog.Error("invalid msgid %d", msgID)
	}
}

func (service *ServiceSdpRpc) HandleError(current *CurrentContent, err error) {
	service.imp.HandleError(current, err)
}

func sdpLen(b []byte) uint32 {
	return uint32(b[3]) | uint32(b[2])<<8 | uint32(b[1])<<16 | uint32(b[0])<<24
}

func (service *ServiceSdpRpc) Unmarshal(sess *Session, data []byte) (lenParsed int, msgID int64, msg interface{}, err error) {
	if len(data) < 4 {
		return 0, 0, nil, nil
	}
	msgLen := sdpLen(data)
	if msgLen < 4 || msgLen >= 1024*1024*1024 {
		return int(msgLen), 0, nil, fmt.Errorf("message length is invalid: %d", msgLen)
	}

	if len(data) < int(msgLen) {
		return 0, 0, nil, nil
	}

	if service.isClient {
		rsp := &SdpRpcRspProto{}
		e := SpbDecode(data[4:msgLen], rsp)
		if e != nil {
			return int(msgLen), 0, nil, e
		}

		service.rpcMutex.Lock()
		v, ok := service.rpcRequests[rsp.RequestId]
		if ok && v.signal != nil { //sync call
			v.signal <- rsp
			service.rpcMutex.Unlock()
			return int(msgLen), -1, nil, nil
		}
		service.rpcMutex.Unlock()
		//async call
		return int(msgLen), 3, rsp, nil
	} else {
		req := &SdpRpcReqProto{}
		e := SpbDecode(data[4:msgLen], req)
		if e != nil {
			return int(msgLen), 0, nil, e
		}
		return int(msgLen), 2, req, nil
	}
}

func (service *ServiceSdpRpc) HashProcessor(current *CurrentContent, msgID uint64, msg interface{}) (processorID int) {
	return service.imp.HashProcessor(current)
}

func (service *ServiceSdpRpc) send(current *CurrentContent, i interface{}) error {
	buf, e := SpbEncode(i)
	if e != nil {
		return e
	}
	buf = PackSdpProtocol(buf)
	return current.Sess.Send(buf, current.Peer)
}

func PackSdpProtocol(data []byte) []byte {
	msglen := len(data) + 4
	sdpMsg := Spb{}
	sdpMsg.packByte(byte(msglen >> 24))
	sdpMsg.packByte(byte(msglen >> 16))
	sdpMsg.packByte(byte(msglen >> 8))
	sdpMsg.packByte(byte(msglen))
	sdpMsg.packData(data)
	return sdpMsg.buf
}
