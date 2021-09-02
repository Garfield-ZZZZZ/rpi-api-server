package utils

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

type Ifttt struct {
	logger *log.Logger
	url    string
}

func GetIfttt(key string, eventName string) *Ifttt {
	if key == "" {
		panic("ifttt key not set")
	}
	if eventName == "" {
		panic("ifttt event name not set")
	}
	var ret = &Ifttt{
		logger: GetLogger("Ifttt"),
		url:    fmt.Sprintf("https://maker.ifttt.com/trigger/%s/with/key/%s", eventName, key),
	}
	ret.logger.Printf("ifttt instance created, target event: %q\n", eventName)
	return ret
}

func (i *Ifttt) Send(title string, message string, priority int) error {
	var payload = fmt.Sprintf("{\"value1\": \"%s\"}", message)
	resp, err := http.Post(i.url, "application/json", bytes.NewBuffer([]byte(payload)))
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusOK {
		return nil
	} else {
		respBytes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.New(string(respBytes))
	}
}
