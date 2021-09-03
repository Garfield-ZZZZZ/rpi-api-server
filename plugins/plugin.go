package plugins

import "net/http"

type Plugin interface {
	Start()
	ServeHTTP(rw http.ResponseWriter, req *http.Request)
}
