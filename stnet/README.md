stnet is a simple net lib.
example
### echo server
```
s:=NewServer("echo",100)
s.AddService("echo","127.0.0.1:6666",ServiceEcho{},1)
s.Start()
```
