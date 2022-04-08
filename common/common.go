package common

import "encoding/base64"

type Config struct {
	ToggleToken       string
	ToggleUserName    string
	ToggleWorkSpaceId string
	JiraUsername      string
	JiraPassword      string
	JiraUrl           string
}

func BasicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}
