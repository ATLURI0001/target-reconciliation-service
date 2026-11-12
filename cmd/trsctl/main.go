// trsctl is a deliberately small local management API client.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func request(args []string) error {
	base := os.Getenv("TRS_API_URL")
	if base == "" {
		base = "http://127.0.0.1:8080"
	}
	origin, e := url.Parse(base)
	if e != nil || origin.Host == "" || origin.User != nil {
		return fmt.Errorf("invalid TRS_API_URL")
	}
	if origin.Scheme != "https" && !(origin.Scheme == "http" && (origin.Hostname() == "127.0.0.1" || origin.Hostname() == "localhost")) {
		return fmt.Errorf("TLS required outside localhost")
	}
	if len(args) == 0 {
		return fmt.Errorf("usage: trsctl snapshot | audit | register ID | policy FILE | approve ID | cancel ID")
	}
	method, path, body := "GET", "/v1/snapshot", []byte(nil)
	switch args[0] {
	case "snapshot":
	case "audit":
		path = "/v1/audit"
	case "register":
		if len(args) != 2 {
			return fmt.Errorf("register requires cluster ID")
		}
		method = "POST"
		path = "/v1/clusters"
		body, _ = json.Marshal(map[string]string{"id": args[1], "environment": "test"})
	case "policy":
		if len(args) != 2 {
			return fmt.Errorf("policy requires JSON file")
		}
		method = "PUT"
		path = "/v1/policy"
		body, e = os.ReadFile(args[1])
		if e != nil {
			return e
		}
	case "approve", "cancel":
		if len(args) != 2 {
			return fmt.Errorf("incident ID required")
		}
		method = "POST"
		path = "/v1/incidents/" + url.PathEscape(args[1]) + "/" + args[0]
	default:
		return fmt.Errorf("unknown command")
	}
	token := os.Getenv("TRS_TOKEN")
	if token == "" {
		return fmt.Errorf("set TRS_TOKEN (viewer for reads, operator for changes)")
	}
	req, e := http.NewRequest(method, strings.TrimRight(base, "/")+path, bytes.NewReader(body))
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return fmt.Errorf("redirect refused") }}
	res, e := client.Do(req)
	if e != nil {
		return fmt.Errorf("API request failed")
	}
	defer res.Body.Close()
	data, e := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if e != nil {
		return e
	}
	if res.StatusCode >= 300 {
		return fmt.Errorf("API returned HTTP %d: %s", res.StatusCode, string(data))
	}
	_, e = os.Stdout.Write(data)
	return e
}
func main() {
	if e := request(os.Args[1:]); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
