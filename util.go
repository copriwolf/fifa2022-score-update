package main

import (
	"errors"
	"fmt"
	"log"
)

// GoWithRecovery go recover panic
func GoWithRecovery(f func()) {
	go func() {
		defer func() {
			if e := recover(); e != nil {
				stack := Stack(3)
				outErr := errors.New(fmt.Sprintf("recover stack: %s, err: %s", stack, e))
				log.Printf(outErr.Error())

				httpPostJson(ErrReportApi, getErrStr(outErr))
			}
		}()
		f()
	}()
}
