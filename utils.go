package main

import (
	"fmt"
	"net/http"
)

func handle(message string, err error) {
	if err == nil {
		return
	}
	logger.Printf("%s %s\n", message, err.Error())
}

func fatal(message string, err error) {
	if err == nil {
		return
	}
	logger.Fatalf("%s %s\n", message, err.Error())
}

func fhandle(w http.ResponseWriter, message string, err error) {
	w.WriteHeader(http.StatusInternalServerError)
	fmt.Fprintf(w, `{"status": false, "err": "%s"}`, message)

	if err == nil {
		return
	}

	logger.Printf("%s %s\n", message, err.Error())
}
