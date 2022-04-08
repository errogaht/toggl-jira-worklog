package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/errogaht/toggl-jira-worklog/common"
	"github.com/errogaht/toggl-jira-worklog/toggl"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

func main() {
	config := getConfig()

	fmt.Println("Give me a date, i'll go to Toggle fetch all logs for the day and create work logs in Jira")

	historyLines := loadHistory()
	printHistory(&historyLines)

	enteredDate := askDate()
	historyLines = append(historyLines, enteredDate)
	saveHistory(&historyLines)

	workLogs := getTogglLogs(enteredDate, config)
	createJiraLogs(workLogs, enteredDate, config)
}

func jiraLogWork(issueId string, secondsSpent uint32, date string, config *common.Config) {
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

func printHistory(historyLines *[]string) {
	fmt.Println("7 days history:")
	lines := *historyLines

	for i := range lines {
		fmt.Println(lines[i])
	}
}

func getHomeDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	return homeDir
}

func getHistoryFilePath() string {
	homeDir := getHomeDir()
	return homeDir + "/.toggl-jira-worklog/history"
}
func loadHistory() (lines []string) {
	historyFile := getHistoryFilePath()
	file, err := os.OpenFile(historyFile, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Fatal(err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(file)

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return
}

func askConfigValue(paramName string) (value string) {
	fmt.Println(paramName + " is not found in config, please enter the value:")
	fmt.Scan(&value)
	return
}

func getConfig() *common.Config {
	var config common.Config
	homeDir := getHomeDir()

	configPath := homeDir + "/.toggl-jira-worklog"

	err := os.MkdirAll(configPath, 0744)
	if err != nil {
		log.Fatal(err)
	}
	configPath += "/config.json"
	file, err := os.OpenFile(configPath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println(err)
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(file)

	b := new(strings.Builder)
	_, err = io.Copy(b, file)
	if err != nil {
		log.Fatal(err)
	}
	fileContent := b.String()
	if fileContent == "" {
		fileContent = "{}"
	}

	err = json.Unmarshal([]byte(fileContent), &config)
	if err != nil {
		log.Fatal(err)
	}

	if config.ToggleToken == "" {
		config.ToggleToken = askConfigValue("ToggleToken")
	}
	if config.ToggleUserName == "" {
		config.ToggleUserName = askConfigValue("ToggleUserName")
	}
	if config.ToggleWorkSpaceId == "" {
		config.ToggleWorkSpaceId = askConfigValue("ToggleWorkSpaceId")
	}
	if config.JiraUrl == "" {
		config.JiraUrl = askConfigValue("JiraUrl (https://company.atlassian.net)")
	}
	if config.JiraUsername == "" {
		config.JiraUsername = askConfigValue("JiraUsername")
	}
	if config.JiraPassword == "" {
		config.JiraPassword = askConfigValue("JiraPassword")
	}

	jsonString, err := json.Marshal(config)
	if err != nil {
		log.Fatal(err)
	}
	_, err = file.Seek(0, 0)
	if err != nil {
		log.Fatal(err)
	}
	_, err = file.WriteString(string(jsonString))
	if err != nil {
		log.Fatal(err)
	}

	return &config
}

func createJiraLogs(workLogs *map[string]uint32, enteredDate string, config *common.Config) {
	for issueId, seconds := range *workLogs {
		fmt.Printf("%d sec. -> %s\n", seconds, issueId)
		jiraLogWork(issueId, seconds, enteredDate, config)
	}
}

func getTogglLogs(enteredDate string, config *common.Config) *map[string]uint32 {
	togglLogs := toggl.Report(enteredDate, config)

	workLogs := make(map[string]uint32)
	for _, togglLog := range togglLogs {
		r, _ := regexp.Compile(`^(\w+-\d+) ?.*$`)
		result := r.FindStringSubmatch(togglLog.Name)
		if result == nil {
			panic(fmt.Sprintf(`Toggle log name is not valid: "%issueId" expected format is "AB-123" or "AB-123 text"`, togglLog.Name))
		}
		workLogs[result[1]] = togglLog.Seconds
	}
	return &workLogs
}

func saveHistory(historyLinesPointer *[]string) {
	historyFile := getHistoryFilePath()
	lines := *historyLinesPointer
	if len(lines) >= 7 {
		lines = lines[len(lines)-7:]
	}

	if err := ioutil.WriteFile(historyFile, []byte(strings.Join(lines, "\n")), 0666); err != nil {
		log.Fatal(err)
	}
}

func askDate() string {
	var enteredDate string
	fmt.Println("Enter date yyyy-mm-dd:")
	_, err := fmt.Scan(&enteredDate)
	if err != nil {
		log.Fatal(err)
	}

	d, err := time.Parse("2006-01-02", enteredDate)
	if err != nil {
		log.Fatal(err)
	}
	enteredDate = d.Format("2006-01-02")

	return enteredDate
}
