package sdp

import (
	. "github.com/gotask/gost/stnet"
)

type SdpServer struct {
	*Server
}

func (svr *SdpServer) AddRpcService(name, address string, rpcFuncStruct interface{}, threadId int) (*Service, error) {
	rpcImp := &RPCServerImp{rpcFuncStruct}
	return svr.AddService(name, address, rpcImp, threadId)
}

func (svr *SdpServer) AddRpcClient(name, servicename, address string, threadId int) (*RPC, error) {
	rpcimp := &RPCImp{}
	ct, err := svr.AddConnect(name, address, 100, rpcimp, threadId)
	if err != nil {
		return nil, err
	}
	rpcimp.conn = ct
	return &RPC{ct, rpcimp, servicename, 1}, nil
}

//AddService ServiceSdp
//AddConnect ConnectSdp
//NewConnect ConnectSdp
