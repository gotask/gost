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
