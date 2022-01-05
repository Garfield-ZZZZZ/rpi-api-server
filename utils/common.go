package utils

import (
	"fmt"
	"log"
	"os"
	"strconv"
)

func GetLogger(name string) *log.Logger {
	return log.New(os.Stdout, fmt.Sprintf("[%s] ", name), log.Ldate|log.Ltime|log.Lmsgprefix)
}

func GetEnvVarString(name string, defaultValue string) string {
	var value, exists = os.LookupEnv(name)
	if exists {
		return value
	} else {
		return defaultValue
	}
}

func GetEnvVarBool(name string, defaultValue bool) bool {
	var strValue = GetEnvVarString(name, "placeholder")
	var ret, err = strconv.ParseBool(strValue)
	if err != nil {
		return defaultValue
	}
	return ret
}

func GetEnvVarInt(name string, defaultValue int) int {
	var strValue = GetEnvVarString(name, "placeholder")
	var ret, err = strconv.Atoi(strValue)
	if err != nil {
		return defaultValue
	}
	return ret
}

type NotificationPusher interface {
	Send(title string, message string, priority int) error
}

func GetNotificationPusher() NotificationPusher {
	var service = GetEnvVarString("PUSH_SERVICE", "")
	switch service {
	case "ifttt":
		var key = GetEnvVarString("IFTTT_KEY", "")
		var eventName = GetEnvVarString("IFTTT_EVENT_NAME", "")
		var ret = GetIfttt(key, eventName)
		return ret
	case "pushover":
		var token = GetEnvVarString("PUSHOVER_TOKEN", "")
		var user = GetEnvVarString("PUSHOVER_USER", "")
		var device = GetEnvVarString("PUSHOVER_DEVICE", "")
		var ret = GetPushover(token, user, device)
		return ret
	case "disabled":
		return nil
	default:
		panic(fmt.Sprintf("invalid push service: %q", service))
	}
}
