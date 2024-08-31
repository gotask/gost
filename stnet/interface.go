package stnet

type BaseServiceInterface interface {
	NewConnect(address string, userdata interface{}) *Connector
	GetSession(id uint64) *Session
}

type JsonServiceInterface interface {
	NewConnect(address string, userdata interface{}) *Connector
	GetSession(id uint64) *Session
	SendJsonCmd(sess *Session, msgID uint64, msg []byte) error
}

type RpcServerInterface interface {
	Push(ps *PushProto) error
}

type RpcClientInterface interface {
	// RpcCall serviceName funcName funcParams callback(could nil) exception(could nil, func(rspCode int32))
	// example RpcCall("saas", "Add", 1, 2, func(result int) {}, func(exception int32) {})
	// example RpcCall("saas1", "Ping")
	RpcCall(serviceName, funcName string, params ...interface{}) error
	RpcCallSync(serviceName, funcName string, params ...interface{}) error
	//CloseConnector close the connector to remote service
	CloseConnector(serviceName string)
}

type RouterServerInterface interface {
	ListService(name string) map[string][]string
}

type RouterClientInterface interface {
	RpcCall(serviceName, funcName string, params ...interface{}) error
	RpcCallSync(serviceName, funcName string, params ...interface{}) error
}
