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

type UrlConfig struct {
	ID         int    `json:"id"`
	URL        string `json:"url"`
	ExpectCode int    `json:"expected_code"`
	MaxTimeOut int    `json:"max_timeout"`
	Content    string `json:"content"`
}

type MyEvent struct {
	URLs []UrlConfig `json:"urls"`
}

type UrlResult struct {
	ID                 int    `json:"id"`
	ReceivedCode       int    `json:"received_code"`
	ExceededMaxTimeOut bool   `json:"exceeded_max_timeout"`
	ContentFound       bool   `json:"content_found"`
	Error              string `json:"error"`
}

type MyResponse struct {
	URLs []UrlResult `json:"urls"`
}

// Status makes a GET request to a given URL and checks whether or not the
// resulting status code is 200.
func Status(siteUrl UrlConfig, wg *sync.WaitGroup) {

	defer wg.Done()

	// Create a new HTTP request
	req, err := http.NewRequest("GET", siteUrl.URL, nil)
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

	if _, err := io.Copy(ioutil.Discard, resp.Body); err != nil {
		log.Fatal(err)
	}

	var score int = 0
	var match int = 0
	var response UrlResult
	response.ID = siteUrl.ID
	response.ReceivedCode = 0
	response.ExceededMaxTimeOut = false
	response.ContentFound = false
	response.Error = "false"
	var now = time.Now()

	if err != nil {
		//response.Error = fmt.Sprintf("%v", err)
		if err2, ok := err.(*url.Error); ok {
			if err3, ok := err2.Err.(net.Error); ok {
				response.ExceededMaxTimeOut = err3.Timeout()
			}
		}
	} else {

		response.ReceivedCode = resp.StatusCode

		defer resp.Body.Close()

		data, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			response.ContentFound = strings.Contains(string(data), siteUrl.Content)
		}

		if response.ContentFound == true {
			match = 1
		}

		if resp.StatusCode == siteUrl.ExpectCode {
			score = 1
		}

	}

	var total_time = result.Total(now)

	/**
	* u: url
	* i: id
	* s: status
	* m: match
	* r: region
	* t: total request time duration (in ms)
	 */
	hackalogURL := fmt.Sprintf("https://hackalog.scalefast.ninja/i?u=%s&i=%s&s=%s&m=%s&r=%s&t=%d", siteUrl.URL, strconv.Itoa(siteUrl.ID), strconv.Itoa(score), strconv.Itoa(match), os.Getenv("AWS_REGION"), int(total_time/time.Millisecond))

	var httpClient = &http.Client{
		Timeout: time.Second * 5,
	}

	httpClient.Get(hackalogURL)
	defer httpClient.CloseIdleConnections()

	/*if err2 != nil {
		fmt.Printf("Hackalog error: %v", err2)
	} else {
		fmt.Println("Hackalog has said: " + string(resp2.StatusCode))
	}*/
}

func checkURLsStatus(URLs []UrlConfig) {
	var wg sync.WaitGroup
	for _, url := range URLs {
		wg.Add(1)
		go Status(url, &wg)
	}
	wg.Wait()
}

func HandleLambdaEvent(event MyEvent) {
	checkURLsStatus(event.URLs)
}

func main() {
	lambda.Start(HandleLambdaEvent)
}
