package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ReportTask struct {
	ProjectKey    string
	ServerUrl     string
	ServerVersion string
	DashboardUrl  string
	CeTaskId      string
	CeTaskUrl     string
}

type CeTaskResult struct {
	Task struct {
		Id           string `json:"id"`
		ComponentKey string `json:"componentKey"`
		Status       string `json:"status"`
	} `json:"task"`
}

type ProjectStatusResult struct {
	ProjectStatus struct {
		Conditions        []map[string]interface{} `json:"conditions"`
		Periods           []map[string]interface{} `json:"periods"`
		IgnoredConditions bool                     `json:"ignoredConditions"`
		Status            string                   `json:"status"`
	} `json:"projectStatus"`
}

func main() {
	var reportTaskPath string
	var waitSeconds uint
	flag.StringVar(&reportTaskPath, "reportTaskPath", ".", "the path of report-task.txt file. when it's empty,program will auto search it")
	flag.UintVar(&waitSeconds, "waitSeconds", 300, "wait time for task analysis")
	flag.Parse()

	reportTaskFile := ""
	filepath.Walk(reportTaskPath, func(path string, info fs.FileInfo, err error) error {
		if reportTaskFile != "" {
			return filepath.SkipDir
		}
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err
		}
		if !info.IsDir() && info.Name() == "report-task.txt" {
			reportTaskFile = filepath.Join(reportTaskPath, path)
			log.Println("detect report file: " + reportTaskFile)
			return filepath.SkipDir
		}
		return nil
	})
	if reportTaskFile == "" {
		log.Fatalln("cannot find report-task.txt")
	}
	reportTask := readFromFile(reportTaskFile)
	log.Println("DashboardUrl: " + reportTask.DashboardUrl)
	waitingTaskCompleted(reportTask, waitSeconds)

	checkProjectStatus(reportTask)
}

func readFromFile(reportTaskFile string) *ReportTask {
	f, err := os.OpenFile(reportTaskFile, os.O_RDONLY, os.ModePerm)
	if err != nil {
		log.Fatalf("open file error: %v", err)
	}
	defer f.Close()

	reportTask := &ReportTask{}
	rd := bufio.NewReader(f)
	for {
		line, err := rd.ReadString('\n')
		if line != "" {
			kv := strings.SplitN(line, "=", 2)
			key := strings.TrimSpace(kv[0])
			value := strings.TrimSpace(kv[1])
			if key == "projectKey" {
				reportTask.ProjectKey = value
			} else if key == "serverUrl" {
				reportTask.ServerUrl = value
			} else if key == "serverVersion" {
				reportTask.ServerVersion = value
			} else if key == "dashboardUrl" {
				reportTask.DashboardUrl = value
			} else if key == "ceTaskId" {
				reportTask.CeTaskId = value
			} else if key == "ceTaskUrl" {
				reportTask.CeTaskUrl = value
			}
		} else if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("read file line error: %v", err)
		}
	}
	return reportTask
}

func waitingTaskCompleted(reportTask *ReportTask, waitSeconds uint) {
	log.Println("CeTaskUrl: " + reportTask.CeTaskUrl)
	timeOutCh := time.After(time.Second * time.Duration(waitSeconds))
	for {
		select {
		default:
			result := getCeTaskResult(reportTask.CeTaskUrl)
			switch result.Task.Status {
			case "SUCCESS":
				log.Println("ce task completed")
				return
			case "PENDING", "IN_PROGRESS":
				log.Println("WAITING......")
				time.Sleep(time.Second * 2)
			default:
				// sonarqube analysis failed
				msg := "sonarqube latest analysis failed ,status " + result.Task.Status
				log.Fatalln(msg)
			}
		case <-timeOutCh:
			log.Fatalf("task timeout after %d seconds", waitSeconds)
		}
	}
}

func getCeTaskResult(ceTaskUrl string) *CeTaskResult {
	response, err := http.Get(ceTaskUrl)
	if err != nil {
		log.Fatalf("get ce task result error: %v", err)
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatalf("get ce task result error: %v", err)
	}
	var s CeTaskResult
	if err = json.Unmarshal(body, &s); err != nil {
		log.Fatalf("get ce task result error: %v", err)
	}
	return &s
}

func checkProjectStatus(reportTask *ReportTask) {
	projectStatusUrl := fmt.Sprintf("%s/api/qualitygates/project_status?projectKey=%s", reportTask.ServerUrl, url.QueryEscape(reportTask.ProjectKey))
	log.Println("ProjectStatusUrl: " + projectStatusUrl)
	response, err := http.Get(projectStatusUrl)
	if err != nil {
		log.Fatalf("get project status result error: %v", err)
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		log.Fatalf("get project status result error: %v", err)
	}
	var s ProjectStatusResult
	if err = json.Unmarshal(body, &s); err != nil {
		log.Fatalf("get project status result error: %v", err)
	}
	if s.ProjectStatus.Status == "OK" {
		log.Println("PASS SONAR GATEWAY CHECK")
		return
	}
	for _, condition := range s.ProjectStatus.Conditions {
		if condition["status"].(string) != "OK" {
			bs, _ := json.Marshal(condition)
			log.Printf("failed metric: %s", string(bs))
		}
	}
	log.Fatalf("SONAR GATEWAY CHECK FAILED WITH STATUS: %s", s.ProjectStatus.Status)
}
