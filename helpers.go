package main

import "log"

func ifErr(e interface{}, msg string) {
	if e != nil {
		log.Printf("[Error] %s with error %s", msg, e)
	}
}

func ifErrFatal(e interface{}, msg string) {
	if e != nil {
		log.Fatalf("[Fatal] %s with error: %s", msg, e)
	}
}
