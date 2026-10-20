package util

import (
	"errors"
	"fmt"
	"runtime"
)

func Errorf(format string, args ...any) error {
	pc, _, _, ok := runtime.Caller(1)
	prefix := "unknown: "
	if ok {
		prefix = fmt.Sprintf("%s: ", runtime.FuncForPC(pc).Name())
	}
	return fmt.Errorf(prefix+format, args...)
}

func NewError(info string) error {
	pc, _, _, ok := runtime.Caller(1)
	prefix := "unknown: "
	if ok {
		prefix = fmt.Sprintf("%s: ", runtime.FuncForPC(pc).Name())
	}
	return errors.New(prefix + info)
}
