package logging

import (
	"fmt"
	"log"
	"os"
)

type Logger struct {
	className string
}

func NewLogger(className string) *Logger {
	return &Logger{className: className}
}

func (l *Logger) Info(format string, args ...interface{}) {
	log.SetOutput(os.Stdout)
	log.Printf("[INFO] [%s] %s", l.className, fmt.Sprintf(format, args...))
}

func (l *Logger) Error(format string, args ...interface{}) {
	log.SetOutput(os.Stderr)
	log.Printf("[ERROR] [%s] %s", l.className, fmt.Sprintf(format, args...))
}

func (l *Logger) Fatal(format string, err error) {
	log.SetOutput(os.Stderr)
	log.Fatal(fmt.Sprintf("[FATAL] [%s] %s: %v", l.className, format, err))
}
