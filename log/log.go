package log

import (
	"log"
	"os"
)

var logger = NewLogger(false)

func GetLogger() *Logger {
	return logger
}

func SetLogger(debugFlag bool) {
	logger = NewLogger(debugFlag)
}

type Logger struct {
	*log.Logger
	debugFlag bool
}

func NewLogger(debugFlag bool) *Logger {
	return &Logger{
		Logger:    log.New(os.Stdout, "", log.Ldate|log.Ltime),
		debugFlag: debugFlag,
	}
}

func (l *Logger) Debug(v ...interface{}) {
	if l.debugFlag {
		l.Println("[DEBUG]", v)
	}
}

func (l *Logger) Debugf(format string, v ...interface{}) {
	if l.debugFlag {
		l.Printf("[DEBUG] "+format+"\n", v...)
	}
}

func (l *Logger) Info(v ...interface{}) {
	l.Println("[INFO]", v)
}

func (l *Logger) Infof(format string, v ...interface{}) {
	l.Printf("[INFO] "+format+"\n", v...)
}

func (l *Logger) Warn(v ...interface{}) {
	l.Println("[WARN]", v)
}

func (l *Logger) Warnf(format string, v ...interface{}) {
	l.Printf("[WARN] "+format+"\n", v...)
}

func (l *Logger) Error(v ...interface{}) {
	l.Println("[ERROR]", v)
}

func (l *Logger) Errorf(format string, v ...interface{}) {
	l.Printf("[ERROR] "+format+"\n", v...)
}
