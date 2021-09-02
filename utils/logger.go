package utils

import (
	"fmt"
	"log"
	"os"
)

func GetLogger(name string) *log.Logger {
	return log.New(os.Stdout, fmt.Sprintf("[%s] ", name), log.Ldate|log.Ltime|log.Lmsgprefix)
}
