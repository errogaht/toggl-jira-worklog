package toggl

import (
	"encoding/json"
	"github.com/errogaht/toggl-jira-worklog/common"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"sort"
)

type ReportRespInnerItemTitle struct {
	TimeEntry string `json:"time_entry"`
}
type ReportRespInnerItem struct {
	Title ReportRespInnerItemTitle
	Time  uint32
}
type ReportRespItem struct {
	Items []ReportRespInnerItem
}
type ReportResp struct {
	Data []ReportRespItem
}

type TaskLog struct {
	Name    string
	Seconds uint32
}

func Report(enteredDate string, config *common.Config) []TaskLog {
	url := "https://api.track.toggl.com/reports/api/v2/summary"

	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", "Basic "+common.BasicAuth(config.ToggleToken, "api_token"))

	q := req.URL.Query()
	q.Add("user_agent", config.ToggleUserName)
	q.Add("workspace_id", config.ToggleWorkSpaceId)
	q.Add("since", enteredDate)
	q.Add("until", enteredDate)
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

	var data ReportResp
	err = json.Unmarshal(body, &data)
	if err != nil {
		panic(err.Error())
	}
	var logs []TaskLog
	for _, item := range data.Data[0].Items {
		logs = append(logs, TaskLog{
			Name:    item.Title.TimeEntry,
			Seconds: uint32(math.Round(float64(item.Time)/1000/60)) * 60,
		})

	}

	sort.SliceStable(logs, func(i, j int) bool {
		return logs[i].Seconds > logs[j].Seconds
	})

	return logs
}
