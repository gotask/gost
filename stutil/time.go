package stutil

import (
	"time"
)

func Unix2Time(sec int64, nsec int64) time.Time {
	return time.Unix(sec, nsec)
}

func Time2Unix(t time.Time) int64 {
	return t.Unix()
}

func TimeFormat(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

func TimeFormatNeno(t time.Time) string {
	return t.Format("2006-01-02 15:04:05.999999999Z07:00")
}

func TimeFormatYMD(t time.Time) string {
	return t.Format("2006-01-02")
}

func TimeParse(format, stime string) (time.Time, error) {
	return time.ParseInLocation(format, stime, time.Local)
}

func TimeParseUTC(format, stime string) (time.Time, error) {
	return time.ParseInLocation(format, stime, time.UTC)
}

func TimeZeroClock(shift time.Duration) time.Time {
	t, _ := time.ParseInLocation("2006-01-02 15:04:05", time.Now().Format("2006-01-02 00:00:00"), time.Local)
	return t.Add(shift)
}
