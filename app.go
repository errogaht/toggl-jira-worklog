package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/errogaht/toggl-jira-worklog/common"
	"github.com/errogaht/toggl-jira-worklog/jira"
	"github.com/errogaht/toggl-jira-worklog/toggl"
	"io"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"
	"time"
)

func printHistory(historyLines *[]string, historyFile string) {
	fmt.Println("7 days history:")

	file, err := os.OpenFile(historyFile, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	var line string
	for scanner.Scan() {
		line = scanner.Text()
		*historyLines = append(*historyLines, line)
		fmt.Println(line)
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}
func debug(f interface{}) {

}

func askConfigValue(paramName string) (value string) {
	fmt.Println(paramName + " is not found in config, please enter the value:")
	fmt.Scan(&value)
	return
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	configPath := homeDir + "/.toggl-jira-worklog"

	err = os.MkdirAll(configPath, 0744)
	if err != nil {
		return
	}
	configPath += "/config.json"
	file, err := os.OpenFile(configPath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		fmt.Println(err)
	}
	defer file.Close()

	b := new(strings.Builder)
	io.Copy(b, file)
	fileContent := b.String()
	if fileContent == "" {
		fileContent = "{}"
	}
	var config common.Config

	err = json.Unmarshal([]byte(fileContent), &config)
	if err != nil {
		return
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
		return
	}
	_, err = file.Seek(0, 0)
	if err != nil {
		return
	}
	_, err = file.WriteString(string(jsonString))
	if err != nil {
		return
	}

	fmt.Println("Give me a date, i'll go to Toggle fetch all logs for the day and create work logs in Jira")

	var historyLines []string
	historyFile := homeDir + "/.toggl-jira-worklog/history"

	printHistory(&historyLines, historyFile)
	enteredDate := askDate()
	historyLines = append(historyLines, enteredDate)
	writeHistory(&historyLines, historyFile)

	workLogs := getTogglLogs(enteredDate, &config)
	createJiraLogs(&workLogs, enteredDate, &config)
}

func createJiraLogs(workLogs *map[string]uint32, enteredDate string, config *common.Config) {
	for issueId, seconds := range *workLogs {
		fmt.Printf("%d sec. -> %s\n", seconds, issueId)
		jira.LogWork(issueId, seconds, enteredDate, config)
	}
}

func getTogglLogs(enteredDate string, config *common.Config) map[string]uint32 {
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
	return workLogs
}

func writeHistory(historyLinesPointer *[]string, historyFile string) {
	historyLines := *historyLinesPointer
	if len(historyLines) >= 7 {
		historyLines = historyLines[len(historyLines)-7:]
	}

	if err := ioutil.WriteFile(historyFile, []byte(strings.Join(historyLines, "\n")), 0666); err != nil {
		log.Fatal(err)
	}
}

func askDate() string {
	var enteredDate string
	fmt.Println("Enter date yyyy-mm-dd:")
	fmt.Scan(&enteredDate)

	d, err := time.Parse("2006-01-02", enteredDate)
	if err != nil {
		panic(err.Error())
	}
	enteredDate = d.Format("2006-01-02")
	return enteredDate
}
