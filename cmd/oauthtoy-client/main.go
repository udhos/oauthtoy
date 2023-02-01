/*
This is the main package for oauthtoy-client.
*/
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	"github.com/udhos/oauthtoy/env"
)

const version = "0.0.0"

func getVersion(me string) string {
	return fmt.Sprintf("%s version=%s runtime=%s GOOS=%s GOARCH=%s GOMAXPROCS=%d",
		me, version, runtime.Version(), runtime.GOOS, runtime.GOARCH, runtime.GOMAXPROCS(0))
}

func main() {

	var showVersion bool
	flag.BoolVar(&showVersion, "version", showVersion, "show version")
	flag.Parse()

	me := filepath.Base(os.Args[0])

	{
		v := getVersion(me)
		if showVersion {
			fmt.Print(v)
			fmt.Println()
			return
		}
		log.Print(v)
	}

	config := clientcredentials.Config{
		TokenURL:     env.String("TOKEN_URL", "http://localhost:8080/oauth/token"),
		ClientID:     "admin",
		ClientSecret: "admin",
	}

	httpClient := http.DefaultClient

	httpClient.Transport = transport()

	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)

	client := config.Client(ctx)

	const interval = time.Second * 2

	for {
		request(client)
		log.Printf("sleeping %v", interval)
		time.Sleep(interval)
	}
}

func transport() http.RoundTripper {
	t := &myTransport{
		t: http.DefaultTransport,
	}
	return t
}

type myTransport struct {
	t http.RoundTripper
}

func (t *myTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.t.RoundTrip(r)
	if err != nil {
		log.Printf("myTransport.RoundTrip: token retrieval intercepted: error: %v", err)
		return resp, err
	}
	body, errRead := io.ReadAll(resp.Body)
	if errRead != nil {
		log.Printf("myTransport.RoundTrip: token retrieval intercepted: read: %v", errRead)
		return resp, err
	}
	resp.Body.Close()                               // close original body reader
	resp.Body = io.NopCloser(bytes.NewBuffer(body)) // provide new body reader
	log.Printf("myTransport.RoundTrip: token retrieval intercepted: body: %s", string(body))
	return resp, err
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
