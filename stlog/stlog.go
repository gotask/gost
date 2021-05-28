//a simple log lib
//some code from code.google.com/p/log4go
//console log is open, file and sock log is close by default
//you can use functin SetxxxLevel open or close the log pattern
//it will only print the log whose level is higher than the pattern's level
package stlog

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"time"
)

type Level int

const (
	SYSTEM Level = iota
	DEBUG
	INFO
	WARNING
	ERROR
	CRITICAL
	CLOSE
)

var (
	levelStrings = [...]string{"SYSM", "DEBG", "INFO", "WARN", "EROR", "CRIT"}
)

func (l Level) String() string {
	if l < 0 || int(l) > len(levelStrings) {
		return "UNKNOWN"
	}
	return levelStrings[int(l)]
}

/****** format ******/
type formatCacheType struct {
	LastUpdateMillSeconds int64
	formatTime            string
}

var formatCache = &formatCacheType{}

func FormatLogRecord(rec *LogRecord) string {
	if rec == nil {
		return "<nil>"
	}
	millSecs := rec.Created.UnixNano() / 1e6
	if formatCache.LastUpdateMillSeconds != millSecs {
		formatCache.LastUpdateMillSeconds = millSecs
		formatCache.formatTime = rec.Created.Format("2006-01-02 15:04:05.000")
	}

	var builder strings.Builder
	builder.WriteString(formatCache.formatTime)
	builder.WriteString("|")
	builder.WriteString(rec.Level.String())
	builder.WriteString("|")
	builder.WriteString(rec.Source)
	builder.WriteString("|")
	builder.WriteString(rec.Message)
	builder.WriteString("\n")
	return builder.String()
}

type LogRecord struct {
	Level   Level     // The log level
	Created time.Time // The time at which the log message was created (nanoseconds)
	Source  string    // The message source
	Message string    // The log message
}

type Logger struct {
	recv chan *LogRecord
	clos chan int
	wait chan int

	term      Level
	file      Level
	fileWrite *FileLogWriter
}

func (log *Logger) intLogf(lvl Level, format string, args ...interface{}) {
	// Determine caller func
	pc, file, lineno, ok := runtime.Caller(3)
	src := ""
	if ok {
		src = fmt.Sprintf("%s:%s:%d", path.Base(file), path.Base(runtime.FuncForPC(pc).Name()), lineno)
	}

	msg := format
	if len(args) > 0 {
		msg = fmt.Sprintf(format, args...)
	}

	// Make the log record
	rec := &LogRecord{
		Level:   lvl,
		Created: time.Now(),
		Source:  src,
		Message: msg,
	}

	select {
	case <-log.clos:
	case log.recv <- rec:
	default:
		fmt.Fprint(os.Stderr, "log buffer is full\n")
	}
}

func (log *Logger) print(lvl Level, arg0 interface{}, args ...interface{}) {
	switch first := arg0.(type) {
	case string:
		// Use the string as a format string
		log.intLogf(lvl, first, args...)
	default:
		// Build a format string so that it will be similar to Sprint
		log.intLogf(lvl, fmt.Sprint(arg0)+strings.Repeat(" %v", len(args)), args...)
	}
}

func (log *Logger) System(arg0 interface{}, args ...interface{}) {
	const (
		lvl = SYSTEM
	)
	log.print(lvl, arg0, args...)
}

func (log *Logger) Debug(arg0 interface{}, args ...interface{}) {
	const (
		lvl = DEBUG
	)
	log.print(lvl, arg0, args...)
}

func (log *Logger) Info(arg0 interface{}, args ...interface{}) {
	const (
		lvl = INFO
	)
	log.print(lvl, arg0, args...)
}

func (log *Logger) Warn(arg0 interface{}, args ...interface{}) {
	const (
		lvl = WARNING
	)
	log.print(lvl, arg0, args...)
}

func (log *Logger) Error(arg0 interface{}, args ...interface{}) {
	const (
		lvl = ERROR
	)
	log.print(lvl, arg0, args...)
}

func (log *Logger) Critical(arg0 interface{}, args ...interface{}) {
	const (
		lvl = CRITICAL
	)
	log.print(lvl, arg0, args...)
}

func (log *Logger) Close() {
	rec := &LogRecord{
		Level: CLOSE,
	}
	log.recv <- rec

	select {
	case <-log.clos:
		return
	default:
		close(log.clos)
	}
	<-log.wait

	if log.fileWrite != nil {
		log.fileWrite.close()
	}
}

func (log *Logger) SetLevel(lvl Level) {
	log.term = lvl
	log.file = lvl
}

func (log *Logger) SetTermLevel(lvl Level) {
	log.term = lvl
}

//等级 文件名 log文件最大值 是否每天滚动 最大备份文件个数
//param: maxsize int (the maxsize of single log file), daily int(is rotate daily), maxbackup int(max count of the backup log files)
func (log *Logger) SetFileLevel(lvl Level, fname string, param ...int) {
	log.file = lvl
	if lvl == CLOSE {
		if log.fileWrite != nil {
			log.fileWrite.close()
		}
		return
	}

	var maxsize, daily, maxbackup int
	if len(param) > 0 {
		maxsize = param[0]
	}
	if len(param) > 1 {
		daily = param[1]
	}
	if len(param) > 2 {
		maxbackup = param[2]
	}
	log.fileWrite = newFileLogger(fname, maxsize, daily, maxbackup)
}

func NewLogger() *Logger {
	log := &Logger{
		recv: make(chan *LogRecord, 1024*1024),
		clos: make(chan int),
		wait: make(chan int),
		term: DEBUG,
		file: CLOSE,
	}

	go func() {
		defer func() {
			close(log.wait)
		}()

		var buff strings.Builder
		tk := time.NewTicker(time.Millisecond * 300)
		count := 0
		for {
			count++
			select {
			case <-tk.C:
				count = 1024
			case rec, ok := <-log.recv:
				if !ok || rec.Level == CLOSE {
					msg := buff.String()
					if msg != "" {
						log.fileWrite.write(buff.String())
					}
					tk.Stop()
					return
				}
				msg := FormatLogRecord(rec)
				if log.term <= rec.Level {
					fmt.Fprint(os.Stdout, msg)
				}

				if log.file <= rec.Level {
					buff.WriteString(msg)
				}
			}

			if count >= 1024 {
				count = 0
				msg := buff.String()
				if msg != "" {
					err := log.fileWrite.write(msg)
					if err != nil {
						fmt.Fprintf(os.Stderr, "log file write error: %s\n", err.Error())
					}
					buff.Reset()
				}
			}
		}
	}()

	return log
}

func NewFileLogger(fname string) *Logger {
	logger := NewLogger()
	logger.SetFileLevel(DEBUG, fname, 1024*1024*1024, 0, 10)
	return logger
}

func NewFileLoggerWithoutTerm(fname string) *Logger {
	logger := NewFileLogger(fname)
	logger.SetTermLevel(CLOSE)
	return logger
}
