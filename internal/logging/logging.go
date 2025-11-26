package logging

import (
	"log"
	"os"
)

var (
	Info  = log.New(os.Stdout, "[INFO] ", log.LstdFlags|log.Lmsgprefix)
	Error = log.New(os.Stderr, "[ERROR] ", log.LstdFlags|log.Lmsgprefix)
)
