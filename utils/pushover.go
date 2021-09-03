package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

type Pushover struct {
	token  string
	user   string
	device string
	logger *log.Logger
}

const pushoverMessageUrl = "https://api.pushover.net/1/messages.json"

func GetPushover(token string, user string, device string) *Pushover {
	if token == "" {
		panic("pushover token not set")
	}
	if user == "" {
		panic("pushover user not set")
	}
	var ret = &Pushover{
		token:  token,
		user:   user,
		device: device,
		logger: GetLogger("Pushover"),
	}
	ret.logger.Printf("pushover instance created, target device: %q", ret.device)
	return ret
}

func (p *Pushover) Send(title string, message string, priority int) error {
	if priority > 2 || priority < -2 {
		return fmt.Errorf("invalid priority: %d", priority)
	}
	var req = pushoverMessageRequest{
		Token:    p.token,
		User:     p.user,
		Message:  message,
		Device:   p.device,
		Title:    title,
		Priority: priority,
	}
	reqPayload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	resp, err := http.Post(pushoverMessageUrl, "application/json", bytes.NewBuffer(reqPayload))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		p.logger.Printf("request failed with response code %d", resp.StatusCode)
	}
	var result = pushoverMessageResponse{}
	defer resp.Body.Close()
	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var respPayload = string(respBytes)
	err = json.Unmarshal(respBytes, &result)
	if err != nil {
		p.logger.Printf("failed to parse response: %q", respPayload)
		return err
	}
	if result.Status != 1 || resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pushover request failed: %q", respPayload)
	}
	p.logger.Printf("message sent")
	return nil
}

type pushoverMessageRequest struct {
	Token    string `json:"token"`
	User     string `json:"user"`
	Message  string `json:"message"`
	Device   string `json:"device,omitempty"`
	Title    string `json:"title,omitempty"`
	Priority int    `json:"priority,omitempty"`
}

type pushoverMessageResponse struct {
	Status    int      `json:"status"`
	RequestId string   `json:"request"`
	Errors    []string `json:"errors,omitempty"`
}
