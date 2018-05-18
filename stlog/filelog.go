package stlog

import (
	"fmt"
	"os"
	"time"
)

// This log writer sends output to a file
type FileLogWriter struct {
	// The opened file
	filename string
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

func newFileLogWriter(fname string, maxsize int, daily int, maxbackup int) (*FileLogWriter, error) {
	log, err := os.OpenFile(fname, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0660)
	if err != nil {
		return nil, err
	}

	size, err := log.Seek(0, os.SEEK_END)
	if err != nil {
		return nil, err
	}

	return &FileLogWriter{
		filename:       fname,
		file:           log,
		maxsize:        int64(maxsize),
		cursize:        size,
		daily:          daily > 0,
		daily_opendate: time.Now().Day(),
		maxbackup:      maxbackup,
	}, nil
}

func (w *FileLogWriter) close() {
	if w.file != nil {
		w.file.Close()
	}
}

func (w *FileLogWriter) write(msg string) error {
	now := time.Now()
	if (w.maxsize > 0 && w.cursize >= w.maxsize) ||
		(w.daily && now.Day() != w.daily_opendate) {
		if err := w.rotate(); err != nil {
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

func (w *FileLogWriter) rotate() error {
	if w.file != nil {
		w.file.Close()
	}

	_, err := os.Lstat(w.filename)
	if err == nil { // file exists
		fname := w.filename
		if w.daily {
			if time.Now().Day() != w.daily_opendate {
				yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
				fname = w.filename + fmt.Sprintf(".%s", yesterday)
			} else {
				fname = w.filename + fmt.Sprintf(".%s", time.Now().Format("2006-01-02"))
			}
		}
		err = renameFiles(fname, w.maxbackup)
		if err != nil {
			return fmt.Errorf("Rotate: %s\n", err)
		}

		w.file.Close()
		err = os.Rename(w.filename, fname+".001")
		if err != nil {
			return fmt.Errorf("Rotate: %s\n", err)
		}
	}

	// Open the log file
	fd, err := os.OpenFile(w.filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0660)
	if err != nil {
		return err
	}
	w.file = fd

	now := time.Now()
	w.daily_opendate = now.Day()

	w.cursize = 0

	return nil
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
