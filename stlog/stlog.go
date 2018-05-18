//a simple log lib
//some code from code.google.com/p/log4go
//console log is open, file and sock log is close by default
//you can use functin SetxxxLevel open or close the log pattern
//it will only print the log whose level is higher than the pattern's level
package stlog

import (
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARNING
	ERROR
	CRITICAL
	CLOSE
)

var (
	levelStrings = [...]string{"DEBG", "INFO", "WARN", "EROR", "CRIT"}
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
	secs := rec.Created.UnixNano() / 1e9
	millSecs := rec.Created.UnixNano() / 1e6 % secs
	cache := *formatCache
	if cache.LastUpdateMillSeconds != millSecs {
		month, day, year := rec.Created.Month(), rec.Created.Day(), rec.Created.Year()
		hour, minute, second := rec.Created.Hour(), rec.Created.Minute(), rec.Created.Second()
		zone, _ := rec.Created.Zone()
		updated := &formatCacheType{
			LastUpdateMillSeconds: millSecs,
			formatTime:            fmt.Sprintf("%04d-%02d-%02d %02d:%02d:%02d:%03d %s", year, month, day, hour, minute, second, millSecs, zone),
		}
		cache = *updated
		formatCache = updated
	}

	return cache.formatTime + "|" + rec.Level.String() + "|" + rec.Source + "|" + rec.Message + "\n"
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
	sock      Level
	sockip    string
	conn      net.Conn
	fileWrite *FileLogWriter
}

func (log *Logger) intLogf(lvl Level, format string, args ...interface{}) {
	// Determine caller func
	pc, _, lineno, ok := runtime.Caller(3)
	src := ""
	if ok {
		src = fmt.Sprintf("%s:%d", runtime.FuncForPC(pc).Name(), lineno)
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

	for {
		select {
		case <-log.clos:
			return
		case log.recv <- rec:
			return
		case <-time.After(300 * time.Millisecond):
			break
		}
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
	close(log.clos)
	<-log.wait

	if log.fileWrite != nil {
		log.fileWrite.close()
	}
	if log.conn != nil {
		log.conn.Close()
	}
}

func (log *Logger) SetTermLevel(lvl Level) {
	log.term = lvl
}

//param: maxsize int (the maxsize of single log file), daily int(is rotate daily), maxbackup int(max count of the backup log files)
func (log *Logger) SetFileLevel(lvl Level, fname string, param ...int) {
	log.file = lvl
	if lvl == CLOSE {
		if log.fileWrite != nil {
			log.fileWrite.close()
		}
		return
	}

	var err error
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
	log.fileWrite, err = newFileLogWriter(fname, maxsize, daily, maxbackup)
	if err != nil {
		fmt.Fprint(os.Stderr, "log file error: %s\n", err)
		return
	}
}

func (log *Logger) SetSockLevel(lvl Level, serverip string) {
	log.sock = lvl
	log.sockip = serverip
	if lvl == CLOSE {
		if log.conn != nil {
			log.conn.Close()
		}
		return
	}

	sock, err := net.Dial("tcp", serverip)
	if err != nil {
		fmt.Fprint(os.Stderr, "log server connect error(%q): %s\n", serverip, err)
		return
	}
	log.conn = sock
}

func NewLogger() *Logger {
	log := &Logger{
		recv: make(chan *LogRecord, 16),
		clos: make(chan int),
		wait: make(chan int),
		term: DEBUG,
		file: CLOSE,
		sock: CLOSE,
	}

	go func() {
		defer func() {
			close(log.wait)
		}()

		for {
			rec, ok := <-log.recv
			if !ok || rec.Level == CLOSE {
				return
			}
			msg := FormatLogRecord(rec)
			if log.term <= rec.Level {
				fmt.Fprint(os.Stdout, msg)
			}

			if log.file <= rec.Level {
				err := log.fileWrite.write(msg)
				if err != nil {
					fmt.Fprint(os.Stderr, "log file write error: %s", err)
				}
			}

			if log.sock <= rec.Level {
				_, err := log.conn.Write([]byte(msg))
				if err != nil {
					fmt.Fprint(os.Stderr, "sock log send error(%q): %s", log.sockip, err)

					log.conn, err = net.Dial("tcp", log.sockip)
					if err != nil {
						fmt.Fprint(os.Stderr, "log server connect error(%q): %s", log.sockip, err)
						continue
					}
					log.conn.Write([]byte(msg))
				}
			}
		}
	}()

	return log
}
