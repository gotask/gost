a simple mmap file lib.
example
```
f, e := os.OpenFile(fpath, os.O_RDWR, 0644)
NewMmap(f, 65536, 65536)
```
