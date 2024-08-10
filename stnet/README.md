stnet is a simple net lib.
example
### rpc server
```go
package main

import (
	"github.com/gotask/gost/stnet"
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
	s.AddRpcService("ht", ":8085", &Test{}, 1)
	//s.RegisterToRouter([]string{"127.0.0.1:8086"}, map[string]string{"test": "127.0.0.1:8085"}, "test8085", "123", 0, 1)
	s.Start()

	s.StopWithSignal()
}

```

### rpc client
```go
package main

import (
	"fmt"
	"time"

	"github.com/gotask/gost/stnet"
)

func main() {
	s := stnet.NewServer(10, 32)
	rpc := s.AddRpcClient(map[string]string{"test":"127.0.0.1:8085"}, 1000, 1)
	//rpc2 := s.AddRouterClient([]string{"127.0.0.1:8086"}, "test1", "123", 1000, 1)

	s.Start()

	for {
		rpc.RpcCall("test", "Add", 1, 2, func(r int) {
			fmt.Println(r)
		}, func(r int32) {
			fmt.Println(r)
		})
		time.Sleep(time.Second)
	}
}

```

### Json Server
```go
package main

import (
	"fmt"
	"github.com/gotask/gost/stnet"
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
	//SendJsonCmd(current.Sess, cmd.CmdId+1, cmd.CmdData)
}
func (j *JsonS) HashProcessor(current *stnet.CurrentContent, cmd stnet.JsonProto) (processorID int) {
	return -1
}

func main() {
	s := stnet.NewServer(10, 32)
	s.AddJsonService("js", ":8086", 0, &JsonS{}, 1)
	s.Start()

	s.StopWithSignal()
}

```
