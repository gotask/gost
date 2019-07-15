a simple mmap file lib.
example
```
//f, e := os.OpenFile(fpath, os.O_RDWR, 0644)
f, e := stmmap.CreateFile(fpath, 65536*2)
stmmap.NewMmap(f, 65536, 65536)
```
