package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
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
func Status(siteUrl UrlConfig, result chan UrlResult) {

	var httpClient = &http.Client{
		Timeout: time.Second * time.Duration(siteUrl.MaxTimeOut),
	}

	resp, err := httpClient.Get(siteUrl.URL)

	var score int = 0
	var response UrlResult
	response.ID = siteUrl.ID
	response.ReceivedCode = 0
	response.ExceededMaxTimeOut = false
	response.ContentFound = false
	response.Error = "false"

	if err != nil {
		response.Error = fmt.Sprintf("%v", err)
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

		if resp.StatusCode == siteUrl.ExpectCode {
			score = 1
		}

	}

	resp2, err2 := httpClient.Get("https://hostserver.com/i?u=" + siteUrl.URL + "&s=" + strconv.Itoa(score))

	if err2 != nil {
		fmt.Printf("Hackalog error: %v", err2)
	} else {
		fmt.Println("Hackalog has said: " + string(resp2.StatusCode))
	}

	result <- response
}

func checkURLsStatus(URLs []UrlConfig) []UrlResult {
	results := make([]UrlResult, len(URLs))
	for k, url := range URLs {
		result := make(chan UrlResult)
		go Status(url, result)
		results[k] = <-result
	}
	return results
}

func HandleLambdaEvent(event MyEvent) (MyResponse, error) {
	return MyResponse{URLs: checkURLsStatus(event.URLs)}, nil
}

func main() {
	lambda.Start(HandleLambdaEvent)
}
