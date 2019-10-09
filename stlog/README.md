stlog is a simple log lib.
example
```
func main() {
	fmt.Println("Hello World!")
	log := stlog.NewLogger()
	log.SetFileLevel(stlog.DEBUG, "sys.log", 1, 1, 30)
	//or log := stlog.NewFileLogger("sys.log")
	for i := 0; i < 10; i++ {
		log.Info("%d %s %s", i, "xxxx", "oooo")
	}
	defer log.Close()
}
```
