/*
This is the main package for oauthtoy-client.
*/
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

func main() {

	config := clientcredentials.Config{
		TokenURL:     "http://localhost:8080/oauth/token",
		ClientID:     "admin",
		ClientSecret: "admin",
	}

	httpClient := http.DefaultClient

	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)

	client := config.Client(ctx)

	const interval = time.Second * 2

	for {
		request(client)
		log.Printf("sleeping %v", interval)
		time.Sleep(interval)
	}
}

func request(client *http.Client) {

	req, errReq := http.NewRequest("GET", "http://localhost:8080/echo", nil)
	if errReq != nil {
		log.Fatalf("request: %v", errReq)
	}

	resp, errDo := client.Do(req)
	if errDo != nil {
		log.Fatalf("do: %v", errDo)
	}

	defer resp.Body.Close()

	body, errRead := io.ReadAll(resp.Body)
	if errRead != nil {
		log.Fatalf("read: %v", errRead)
	}

	fmt.Printf("response: %s", string(body))
}
