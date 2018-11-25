shm is a simple share memory lib.
example
```
s, e := ShmGet(0x07ee0005, 100*1024*1024)
if s != nil {
	fmt.Println(s.Key())
	s.Detach()
	s.Delete()
}
```
