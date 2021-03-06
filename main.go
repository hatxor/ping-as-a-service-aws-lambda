package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/tcnksm/go-httpstat"
)

type URLConfig struct {
	ID         int    `json:"id"`
	URL        string `json:"url"`
	ExpectCode int    `json:"expected_code"`
	MaxTimeOut int    `json:"max_timeout"`
	Content    string `json:"content"`
}

type Event struct {
	URLs []URLConfig `json:"urls"`
}

type URLResult struct {
	ID                 int    `json:"id"`
	ReceivedCode       int    `json:"received_code"`
	ExceededMaxTimeOut bool   `json:"exceeded_max_timeout"`
	ContentFound       bool   `json:"content_found"`
	Error              string `json:"error"`
}

// Status makes a GET request to a given URL and checks whether or not the
// resulting status code is 200.
func Status(siteURL URLConfig, wg *sync.WaitGroup) {

	defer wg.Done()

	// Create a new HTTP request
	req, err := http.NewRequest("GET", siteURL.URL, nil)
	if err != nil {
		log.Fatal(err)
	}
	// Create a httpstat powered context
	var result httpstat.Result
	ctx := httpstat.WithHTTPStat(req.Context(), &result)
	req = req.WithContext(ctx) // Send request by default HTTP client
	client := http.DefaultClient
	resp, err := client.Do(req)

	if err != nil {
		log.Println(err)
		return
	}

	defer resp.Body.Close()

	if _, err := io.Copy(ioutil.Discard, resp.Body); err != nil {
		log.Fatal(err)
	}

	var score int = 0
	var match int = 0
	var response URLResult
	response.ID = siteURL.ID
	response.ReceivedCode = 0
	response.ExceededMaxTimeOut = false
	response.ContentFound = false
	response.Error = "false"

	if err != nil {
		if err2, ok := err.(*url.Error); ok {
			if err3, ok := err2.Err.(net.Error); ok {
				response.ExceededMaxTimeOut = err3.Timeout()
			}
		}
		return
	}

	response.ReceivedCode = resp.StatusCode

	data, err := ioutil.ReadAll(resp.Body)
	var now = time.Now()
	result.End(now)
	var totalTime = result.Total(now)

	if err == nil {
		response.ContentFound = strings.Contains(string(data), siteURL.Content)
	}

	if response.ContentFound == true {
		match = 1
	}

	if resp.StatusCode == siteURL.ExpectCode {
		score = 1
	}

	/**
	* u: url
	* i: id
	* s: status
	* m: match
	* r: region
	* t: total request time duration (in ms)
	 */
	hackalogURL := fmt.Sprintf("https://hackalog.scalefast.ninja/i?u=%s&i=%s&s=%s&m=%s&r=%s&t=%d", siteURL.URL, strconv.Itoa(siteURL.ID), strconv.Itoa(score), strconv.Itoa(match), os.Getenv("AWS_REGION"), int(totalTime/time.Millisecond))

	var httpClient = &http.Client{
		Timeout: time.Second * 5,
	}

	httpClient.Get(hackalogURL)
	defer httpClient.CloseIdleConnections()

}

func checkURLsStatus(URLs []URLConfig) {
	var wg sync.WaitGroup
	for _, url := range URLs {
		wg.Add(1)
		go Status(url, &wg)
	}
	wg.Wait()
}

func HandleLambdaEvent(event Event) {
	checkURLsStatus(event.URLs)
}

func main() {
	lambda.Start(HandleLambdaEvent)
}
