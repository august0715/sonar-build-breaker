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
	log.Println("CeTaskUrl: " + reportTask.CeTaskUrl)

	timeOutCh := time.After(time.Second * time.Duration(waitSeconds))

	for {
		select {
		default:
			result := getCeTaskResult(reportTask.CeTaskUrl)
			switch result.Task.Status {
			case "SUCCESS":
				log.Println("SUCCESS")
				return
			case "PENDING", "IN_PROGRESS":
				log.Println("WAITING......")
				time.Sleep(time.Second * 2)
			default:
				// sonarqube扫描失败
				msg := "sonarqube latest analysis failed ,status " + result.Task.Status
				log.Fatalln(msg)
			}
		case <-timeOutCh:
			log.Fatalf("task timeout after %d seconds", waitSeconds)
		}
	}
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
		if err == io.EOF {
			break
		} else if err != nil {
			log.Fatalf("read file line error: %v", err)
		}
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
	}
	return reportTask
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
