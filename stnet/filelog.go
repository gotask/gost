package stnet

import (
	"fmt"
	"os"
	"time"
)

// FileLogWriter This log writer sends output to a file
type FileLogWriter struct {
	// The opened file
	filename string
	basename string
	file     *os.File

	// Rotate at size
	maxsize int64
	cursize int64

	// Rotate daily
	daily          bool
	daily_opendate int

	// Keep old logfiles (.001, .002, etc)
	maxbackup int
}

func newFileLogger(fname string, maxsize int, daily int, maxbackup int) *FileLogWriter {
	return &FileLogWriter{
		filename:       fname,
		basename:       fname,
		maxsize:        int64(maxsize),
		daily:          daily > 0,
		daily_opendate: time.Now().Day(),
		maxbackup:      maxbackup,
	}
}

func (w *FileLogWriter) newFileWriter() error {
	now := time.Now()
	w.daily_opendate = now.Day()
	if w.daily {
		w.filename = w.basename + "." + now.Format("20060102")
	}

	log, err := os.OpenFile(w.filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0660)
	if err != nil {
		return err
	}

	size, err := log.Seek(0, os.SEEK_END)
	if err != nil {
		return err
	}

	w.file = log
	w.cursize = size

	return nil
}

func (w *FileLogWriter) close() {
	if w.file != nil {
		w.file.Close()
	}
}

func (w *FileLogWriter) write(msg string) error {
	if w.file == nil {
		err := w.newFileWriter()
		if err != nil {
			fmt.Fprintf(os.Stderr, "log file error: %s\n", err)
			return err
		}
	}

	now := time.Now()
	day := false
	oversize := false
	if w.daily && now.Day() != w.daily_opendate {
		day = true
	}
	if w.maxsize > 0 && w.cursize >= w.maxsize {
		oversize = true
	}
	if day || oversize {
		err := w.rotate(oversize)
		if err != nil {
			return err
		}
	}

	n, err := fmt.Fprint(w.file, msg)
	if err != nil {
		return err
	}
	w.file.Sync()

	w.cursize += int64(n)
	return nil
}

func (w *FileLogWriter) rotate(oversize bool) error {
	if w.file != nil {
		w.file.Close()
	}
	w.file = nil

	if oversize {
		renameFiles(w.filename, w.maxbackup)
	}

	return w.newFileWriter()
}

func renameFiles(name string, maxFiles int) error {
	if maxFiles < 2 {
		return nil
	}
	for i := maxFiles - 1; i > 1; i-- {
		toPath := name + fmt.Sprintf(".%03d", i)
		fromPath := name + fmt.Sprintf(".%03d", i-1)
		if err := os.Rename(fromPath, toPath); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	if err := os.Rename(name, name+".001"); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
