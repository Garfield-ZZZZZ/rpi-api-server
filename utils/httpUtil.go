package utils

import (
	"fmt"
	"io"
	"log"
	"net/http"
)

type HttpHandleFunc func(rw http.ResponseWriter, req *http.Request)

type HttpMethodMux struct {
	GetHandler     HttpHandleFunc
	HeadHandler    HttpHandleFunc
	PostHandler    HttpHandleFunc
	PutHandler     HttpHandleFunc
	PatchHandler   HttpHandleFunc
	DeleteHandler  HttpHandleFunc
	ConnectHandler HttpHandleFunc
	OptionsHandler HttpHandleFunc
	TraceHandler   HttpHandleFunc
	logger         *log.Logger
}

func (h HttpMethodMux) Handle(pattern string) {
	h.logger = GetLogger("HttpMethodMux")
	http.Handle(pattern, h)
}

func (h HttpMethodMux) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	h.logger.Printf("Got new request from %q requesting for %q using method %s", req.RemoteAddr, req.RequestURI, req.Method)
	var handler HttpHandleFunc = nil
	switch req.Method {
	case http.MethodConnect:
		handler = h.ConnectHandler
	case http.MethodDelete:
		handler = h.DeleteHandler
	case http.MethodGet:
		handler = h.GetHandler
	case http.MethodHead:
		handler = h.HeadHandler
	case http.MethodOptions:
		handler = h.OptionsHandler
	case http.MethodPatch:
		handler = h.PatchHandler
	case http.MethodPost:
		handler = h.PostHandler
	case http.MethodPut:
		handler = h.PutHandler
	case http.MethodTrace:
		handler = h.TraceHandler
	}
	if handler != nil {
		handler(rw, req)
	} else {
		h.logger.Printf("unregistered method")
		rw.WriteHeader(http.StatusBadRequest)
		io.WriteString(rw, fmt.Sprintf("invalid method: %s\n", req.Method))
	}
}
