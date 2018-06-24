package stutil

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// file modified time
func FileModTime(path string) int64 {
	f, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return f.ModTime().Unix()
}

// file size bytes
func FileSize(path string) int64 {
	f, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return f.Size()
}

//file lines
func FileLineNum(path string) (num int64) {
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()

	buf := bufio.NewReader(f)
	for {
		_, isPrefix, e := buf.ReadLine()
		for isPrefix {
			_, isPrefix, e = buf.ReadLine()
		}
		if e != nil {
			return
		}
		num++
	}
	return
}

// delete file
func FileDelete(path string) error {
	return os.Remove(path)
}

// rename file
func FileRename(path string, to string) error {
	return os.Rename(path, to)
}

// is file
func IsFile(path string) bool {
	f, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !f.IsDir()
}

// is exist
func FileIsExist(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

//make dir
func FileMakeDir(path string) error {
	return os.MkdirAll(path, os.ModePerm)
}

//create file
func FileCreate(file string) (*os.File, error) {
	d, f := path.Split(file)
	if f == "" {
		return nil, fmt.Errorf("not a file")
	}
	if d != "" {
		err := os.MkdirAll(d, os.ModePerm)
		if err != nil {
			return nil, err
		}
	}
	return os.Create(file)
}

//open appending file
func FileOpenAppend(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_APPEND, 0666)
}

//create a new file and write content
func FileCreateAndWrite(path, content string) error {
	f, err := FileCreate(path)
	if err != nil {
		return err
	}
	if _, err = f.WriteString(content); err != nil {
		return err
	}
	f.Close()

	return nil
}

//append string to file
func FileWriteAndAppend(path, content string) error {
	if FileIsExist(path) {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		if _, err = f.WriteString(content); err != nil {
			return err
		}
		f.Close()
		return nil
	} else {
		return FileCreateAndWrite(path, content)
	}
}

//reand file all content
func FileReadAll(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	c, err := ioutil.ReadAll(f)
	if err != nil && err != io.EOF {
		return "", err
	}
	return string(c), nil
}

//read line
func FileIterateLine(path string, callback func(num int, line string) bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	buf := bufio.NewReader(f)
	n := 0
	for {
		n++
		line, isPrefix, err := buf.ReadLine()
		for isPrefix {
			var next []byte
			next, isPrefix, err = buf.ReadLine()
			line = append(line, next...)
		}
		if err != nil {
			if err == io.EOF {
				return nil
			} else {
				return err
			}
		}
		if !callback(n, string(line)) {
			break
		}
	}
	return nil
}

//walk files
func FileIterateDir(path, filter string, callback func(file string) bool) error {
	err := filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
		if f == nil {
			return err
		}
		if f.IsDir() {
			//return filepath.SkipDir
			return nil
		}
		if filter != "" {
			if !strings.HasSuffix(path, filter) {
				return nil
			}
		}
		if !callback(path) {
			return fmt.Errorf("walk file over")
		}
		return nil
	})

	return err
}

func FileStdinHasData() bool {
	stat, statErr := os.Stdin.Stat()

	if statErr != nil {
		return false
	}

	if (stat.Mode() & os.ModeCharDevice) == 0 {
		return true
	} else {
		return false
	}
}

type STFile struct {
	path string
	file *os.File
	buf  *bufio.Reader
	line int
}

func NewSTFile(path string) (*STFile, error) {
	var fp *os.File
	var err error

	if path == "stdin" {
		fp = os.Stdin
		err = nil
	} else if path == "stdout" {
		fp = os.Stdout
		err = nil
	} else {
		fp, err = os.Open(path)
		if err != nil {
			return nil, err
		}
	}

	return &STFile{path, fp, bufio.NewReader(fp), 0}, nil
}

func (f *STFile) ReadLine() (string, int) {
	if f.file == nil {
		return "", -1
	}

	f.line++
	line, isPrefix, err := f.buf.ReadLine()
	for isPrefix {
		var next []byte
		next, isPrefix, err = f.buf.ReadLine()
		line = append(line, next...)
	}
	if err != nil {
		f.Close()
		return "", -1
	}
	return string(line), f.line
}

func (f *STFile) Close() {
	if f.file == nil {
		return
	}
	f.file.Close()
	f.file = nil
}
