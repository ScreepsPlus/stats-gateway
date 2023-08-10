package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
)

var backendUrl string = os.Getenv("BACKEND_URL")

var startOfQuery string = "data"
var backendStartOfQuery string = "data"

func main() {
	url, _ := url.Parse(backendUrl)
	proxy := httputil.NewSingleHostReverseProxy(url)
	h := http.NewServeMux()
	h.Handle("/metrics/find", Find(proxy))
	h.Handle("/tags/autoComplete/tags", Tags(proxy))
	h.Handle("/render", Render(proxy))
	h.Handle("/functions", Functions(proxy))
	log.Fatal(http.ListenAndServe(":8181", h))
	log.Println("Listening on :8181")
}

func Find(next http.Handler) http.Handler {
	ourFunc := func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		query := r.Form.Get("query")
		if query == "*" || query == startOfQuery {
			w.Header().Add("Content-Type", "application/json")
			w.WriteHeader(200)
			b := MetricMap([]string{startOfQuery}, "")
			w.Write(b)
			return
		}
		orgId := r.Header.Get("X-Grafana-Org-Id")

		valid := false
		if strings.HasPrefix(query, fmt.Sprintf("%s.", startOfQuery)) {
			if query == fmt.Sprintf("%s.*", startOfQuery) || strings.HasPrefix(query, fmt.Sprintf("%s.*", startOfQuery)) {
				query = strings.Replace(query, "*", orgId+".*", 1)
			} else if strings.HasPrefix(query, fmt.Sprintf("%s.", startOfQuery)) {
				query = strings.Replace(query, fmt.Sprintf("%s", startOfQuery), fmt.Sprintf("%s.", startOfQuery)+orgId, 1)
			}

			query := strings.Replace(query, startOfQuery, backendStartOfQuery, 1)

			switch r.Method {
			case http.MethodPost:
				r.PostForm.Set("query", query)
			case http.MethodGet:
				vals := r.URL.Query()
				vals.Set("query", query)
				r.URL.RawQuery = vals.Encode()
			}
			valid = true
		}

		if valid {
			log.Printf("200: %s, %v", query, orgId)
			if r.Method == http.MethodPost {
				str := r.PostForm.Encode()
				r.Body = ioutil.NopCloser(strings.NewReader(str))
				r.Header.Set("Content-Length", strconv.Itoa(len(str)))
				r.ContentLength = int64(len(str))
			}
			next.ServeHTTP(w, r)
		} else {
			log.Printf("403: %s, %v", query, orgId)
			w.WriteHeader(403)
		}
	}
	return http.HandlerFunc(ourFunc)
}

func Render(next http.Handler) http.Handler {
	ourFunc := func(w http.ResponseWriter, r *http.Request) {
		orgId := r.Header.Get("X-Grafana-Org-Id")

		r.ParseForm()
		targets := r.Form["target"]
		validTargets := make([]string, 0)
		for _, target := range targets {
			valid := false
			target := strings.Replace(target, startOfQuery, fmt.Sprintf("%s.%s", backendStartOfQuery, orgId), 1)

			val := fmt.Sprintf("%s.%s", backendStartOfQuery, orgId)
			if strings.Contains(target, val) {
				valid = true
			}
			if !valid && strings.Contains(target, fmt.Sprintf("%s.*", startOfQuery)) {
				target = strings.Replace(target, fmt.Sprintf("%s.*", startOfQuery), fmt.Sprintf("%s.%s", startOfQuery, orgId), -1)
				valid = true
			}
			if valid {
				validTargets = append(validTargets, target)
			}
		}

		r.Form["target"] = validTargets
		body := r.Form.Encode()
		r.Body = ioutil.NopCloser(bytes.NewBufferString(body))
		r.ContentLength = int64(len(body))
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(ourFunc)
}

func Functions(next http.Handler) http.Handler {
	ourFunc := func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(ourFunc)
}
func Tags(next http.Handler) http.Handler {
	ourFunc := func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	}
	return http.HandlerFunc(ourFunc)
}

func MetricMap(list []string, base string) []byte {
	ret := make([]*GraphiteMetric, 0, 0)
	for _, v := range list {
		id := v
		if base != "" {
			id = fmt.Sprintf("%s.%s", base, v)
		}
		obj := &GraphiteMetric{}
		obj.AllowChildren = 1
		obj.Expandable = 1
		obj.Id = id
		obj.Leaf = 0
		obj.Text = v
		ret = append(ret, obj)
	}
	resp, _ := json.Marshal(ret)
	return resp
}

type GraphiteMetric struct {
	AllowChildren int    `json:"allowChildren"`
	Expandable    int    `json:"expandable"`
	Id            string `json:"id"`
	Leaf          int    `json:"leaf"`
	Text          string `json:"text"`
}
