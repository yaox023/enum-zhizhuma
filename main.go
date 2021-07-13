package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// const testYesURL = "https://mp.zhizhuma.com/book.htm?id=2003"
// const testNoURL = "https://mp.zhizhuma.com/book.htm?id=1004"

type Result struct {
	ID       string `json:"id"`
	OK       bool   `json:"ok"`
	BookName string `json:"bookName"`
	Reason   string `json:"reason"`
}

var (
	httpClient *http.Client
	resultChan chan Result
	taskChan   chan string
)

const (
	MaxIdleConnections int = 10
	RequestTimeout     int = 5
	baseURL                = "https://mp.zhizhuma.com/book.htm?id="
	resultFilePath         = "./result.json"
	maxID                  = 10000 * 30
)

func createHTTPClient() *http.Client {
	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConnsPerHost: MaxIdleConnections,
		},
		Timeout: time.Duration(RequestTimeout) * time.Second,
	}
	return client
}

func request(client *http.Client, targetID string) {
	targetURL := baseURL + targetID
	// fmt.Println("Request: ", targetURL)
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		resultChan <- Result{targetID, false, "", err.Error()}
		<-taskChan
		return
	}
	response, err := httpClient.Do(req)
	if err != nil {
		resultChan <- Result{targetID, false, "", err.Error()}
		<-taskChan
		return
	}

	if response.StatusCode != 200 {
		resultChan <- Result{targetID, false, "", response.Status}
		<-taskChan
		return
	}

	doc, err := goquery.NewDocumentFromReader(response.Body)
	if err != nil {
		resultChan <- Result{targetID, false, "", err.Error()}
		<-taskChan
		return
	}
	response.Body.Close()

	nodes := doc.Find(".tips_des").Nodes
	if len(nodes) != 0 {
		resultChan <- Result{targetID, false, "", "No Page"}
		<-taskChan
		return
	}

	bookName := doc.Find("#book_name").Text()
	resultChan <- Result{targetID, true, bookName, ""}
	<-taskChan
}

func createIDs() [maxID]string {
	var ids [maxID]string
	for i := 1; i <= maxID; i++ {
		ids[i-1] = strconv.Itoa(i)
	}
	rand.Seed(time.Now().UnixNano())
	for i := maxID - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		ids[i], ids[j] = ids[j], ids[i]
	}
	return ids
}

func processResult(wg *sync.WaitGroup) {
	file, err := os.Create(resultFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	writer := io.Writer(file)
	encoder := json.NewEncoder(writer)

	for {
		result := <-resultChan
		err = encoder.Encode(result)
		fmt.Println("Process result: ", result)
		if err != nil {
			fmt.Println("encode json error", err.Error())
		}
		wg.Done()
	}
}

func main() {
	httpClient = createHTTPClient()

	// 用于传递结果
	resultChan = make(chan Result, 10000)

	// 用于控制并发
	taskChan = make(chan string, 50)

	var wg sync.WaitGroup

	go processResult(&wg)

	ids := createIDs()

	for i := 0; i < len(ids); i++ {
		wg.Add(1)
		taskChan <- ids[i]
		go request(httpClient, ids[i])
	}

	wg.Wait()
}
