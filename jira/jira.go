package jira

import (
	"fmt"
	"github.com/errogaht/toggl-jira-worklog/common"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

func LogWork(issueId string, secondsSpent uint32, date string, config *common.Config) {
	url := config.JiraUrl + "/rest/api/latest/issue/" + issueId + "/worklog"

	reqBody := fmt.Sprintf(`{"started": "%s", "timeSpentSeconds": %d}`, date+"T00:00:00.000+0000", secondsSpent)
	req, err := http.NewRequest("POST", url, strings.NewReader(reqBody))
	req.Header.Add("Authorization", "Basic "+common.BasicAuth(config.JiraUsername, config.JiraPassword))
	req.Header.Add("content-type", "application/json")

	q := req.URL.Query()
	q.Add("notifyUsers", "0")
	req.URL.RawQuery = q.Encode()

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err.Error())
	}

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			panic(err.Error())
		}
	}(resp.Body)

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err.Error())
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		panic(fmt.Sprintf("status code: %d, body: %s", resp.StatusCode, body))
	}
}
