package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/errogaht/toggl-jira-worklog/common"
	"github.com/errogaht/toggl-jira-worklog/toggl"
)

func main() {
	monthlyReport := flag.Bool("monthly-report", false, "Show monthly report for the current year")
	flag.Parse()

	config := getConfig()

	if *monthlyReport {
		generateMonthlyReport(config)
		return
	}

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

func generateMonthlyReport(config *common.Config) {
	now := time.Now()
	year := now.Year()
	if now.Month() == 1 && now.Day() == 1 {
		year-- // handle edge case for Jan 1st
	}

	fmt.Printf("Monthly work report for %d\n", year)
	fmt.Println("Month | Required | Worked | Balance")
	fmt.Println("--------------------------------------")

	totalRequired := 0
	totalWorked := 0

	for month := 1; month <= int(now.Month()); month++ {
		start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, -1)
		if month == int(now.Month()) {
			end = now
		}

		workingDays := 0
		for d := start; d.Before(end) || d.Equal(end); d = d.AddDate(0, 0, 1) {
			if d.Weekday() >= time.Monday && d.Weekday() <= time.Friday {
				workingDays++
			}
		}
		required := workingDays * 8 * 3600
		logs := toggl.ReportRange(start.Format("2006-01-02"), end.Format("2006-01-02"), config)
		worked := 0
		for _, log := range logs {
			worked += int(log.Seconds)
		}
		balance := worked - required
		totalRequired += required
		totalWorked += worked

		fmt.Printf("%5s | %7s | %6s | %+7s\n",
			start.Format("Jan"),
			secondsToHoursMinutes(required),
			secondsToHoursMinutes(worked),
			secondsToHoursMinutes(balance),
		)
	}
	fmt.Println("--------------------------------------")
	fmt.Printf("Total | %7s | %6s | %+7s\n",
		secondsToHoursMinutes(totalRequired),
		secondsToHoursMinutes(totalWorked),
		secondsToHoursMinutes(totalWorked-totalRequired),
	)
}

func secondsToHoursMinutes(seconds int) string {
	h := seconds / 3600
	m := (seconds % 3600) / 60
	return fmt.Sprintf("%dh %02dm", h, m)
}
