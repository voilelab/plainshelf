package util

import (
	"log"
	"runtime"
)

func GetCallerName() string {
	pc, _, _, ok := runtime.Caller(2)
	if !ok {
		return "unknown"
	}
	return runtime.FuncForPC(pc).Name()
}

func LogCallStack() {
	const maxDepth = 10
	for i := 2; i < maxDepth+2; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}
		log.Printf("%s:%d - %s\n", file, line, fn.Name())
	}
}
