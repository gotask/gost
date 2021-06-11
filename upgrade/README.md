### update
```
func main() {
	Update()
}
```

### server
```
func main() {
	StartServer("0.0.0.0:1111")
	WaitStopSignal(func() {
		//onclosse
	})
}
```

### client
```
func main() {
	StartClient("0.0.0.0:1111", 0)
	WaitStopSignal(func() {
		//onclosse
	})
}
```