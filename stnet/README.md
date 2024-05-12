stnet is a simple net lib.
example
### rpc server
```go
package main

import (
	"github.com/gotask/gost/stnet"
	"time"
)

type Test struct {
}

func (t *Test) Loop() {

}
func (t *Test) HandleError(current *stnet.CurrentContent, err error) {

}

func (t *Test) HashProcessor(current *stnet.CurrentContent) (processorID int) {
	return -1
}

func (t *Test) Add(a, b int) int {
	return a + b
}

func main() {
	s := stnet.NewServer(10, 32)
	rpc := stnet.NewServiceRpc(&Test{})
	s.AddRpcService("ht", ":8085", 0, rpc, 0)
	s.Start()

	for {
		time.Sleep(time.Hour)
	}
}

```

### rpc client
```go
func main() {
	s := stnet.NewServer(10, 32)
	rpc := stnet.NewServiceRpc(&Test{})
	svr, e := s.AddRpcService("ht", "", 0, rpc, 0)
	if e != nil {
		fmt.Println(e)
		return
	}
	c := svr.NewConnect("127.0.0.1:8085", nil)
	s.Start()

	for {
		rpc.RpcCall(c.Session(), "Add", 1, 2, func(r int) {
			fmt.Println(r)
		}, func(r int32) {
			fmt.Println(r)
		})
		time.Sleep(time.Second)
	}
}

```

### SpbServer
```go
package main

import (
	"fmt"
	"time"

	"github.com/gotask/gost/stnet"
)

type ProA struct {
	A int
	S string
}

type Test struct {
}

func (t *Test) Init() bool {
	return true
}

func (t *Test) Loop() {

}
func (t *Test) Handle(current *stnet.CurrentContent, cmdId uint64, cmd interface{}, e error) {
	//fmt.Println(cmdId, cmd)
	if e != nil {
		fmt.Println(e)
	} else if cmdId == 1{
		fmt.Printf("%+v\n", cmd.(*ProA))
	}else if cmdId == 2 {
		fmt.Println(*cmd.(*string)) 
	}
}

func (t *Test) HashProcessor(current *stnet.CurrentContent, cmdId uint64) (processorID int) {
	return -1
}

func main() {
	s := stnet.NewServer(10, 32)
	ser := stnet.NewServiceSpb(&Test{})
	
	//register msg
	ser.RegisterMsg(1, ProA{})
	var t string = "123"
	ser.RegisterMsg(2, t)
	
	s.AddSpbService("", ":6060", 0, ser, 0)
	//c := svr.NewConnect("127.0.0.1:6060", nil)
	s.Start()
	time.Sleep(time.Hour)
}
```

### Json Server
```go
package main

import (
	"fmt"
	"github.com/gotask/gost/stnet"
	"time"
)

type JsonS struct {
}

func (j *JsonS) Init() bool {
	return true
}
func (j *JsonS) Loop() {

}
func (j *JsonS) Handle(current *stnet.CurrentContent, cmd stnet.JsonProto, e error) {
	fmt.Println(cmd, e)
	//stnet.SendJsonCmd(current.Sess, cmd.CmdId+1, cmd.CmdData)
}
func (j *JsonS) HashProcessor(current *stnet.CurrentContent, cmd stnet.JsonProto) (processorID int) {
	return -1
}

func main() {
	s := stnet.NewServer(10, 32)
	s.AddJsonService("js", ":8086", 0, &JsonS{}, 1)
	s.Start()

	for {
		time.Sleep(time.Hour)
	}
}

```
