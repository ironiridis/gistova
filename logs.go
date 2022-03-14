package gistova

import (
	"fmt"
	"io"
	"os"
	"time"
)

type Logger interface {
	Logln(string)
	Logf(string, ...interface{})
	Errorln(string)
	Errorf(string, ...interface{})
}

type defaultLogger struct {
	stdout  io.Writer
	stderr  io.Writer
	timefmt string
	linefmt string
}

func (l *defaultLogger) Time() string {
	return time.Now().Format(l.timefmt)
}

func (l *defaultLogger) Logln(s string) {
	fmt.Fprintf(l.stdout, l.linefmt, l.Time(), "LOG", s)
}

func (l *defaultLogger) Logf(f string, args ...interface{}) {
	l.Logln(fmt.Sprintf(f, args...))
}

func (l *defaultLogger) Errorln(s string) {
	fmt.Fprintf(l.stderr, l.linefmt, l.Time(), "ERROR", s)
}

func (l *defaultLogger) Errorf(f string, args ...interface{}) {
	l.Errorln(fmt.Sprintf(f, args...))
}

func DefaultLogger() Logger {
	return &defaultLogger{
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		timefmt: time.RFC3339,
		linefmt: "%s\t%s\t%s\n",
	}
}
