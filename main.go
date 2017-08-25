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
	"strings"
)

var backendUrl string = os.Getenv("BACKEND_URL")
var grafanaUrl string = os.Getenv("GRAFANA_URL")

func main() {
	url, _ := url.Parse(backendUrl)
	proxy := httputil.NewSingleHostReverseProxy(url)
	h := http.NewServeMux()
	h.Handle("/metrics/find/",Find(proxy))
	h.Handle("/render",Render(proxy))
    log.Fatal(http.ListenAndServe(":8181", h))
}

func Find(next http.Handler) http.Handler {
    ourFunc := func(w http.ResponseWriter, r *http.Request) {
    	query := r.URL.Query().Get("query")
    	if query == "*" {
    		w.Header().Add("Content-Type","application/json")
    		w.WriteHeader(200)
    		b := MetricMap([]string{"screeps"}, "")
    		w.Write(b)
    		return
    	}
    	cookie := r.Header.Get("Cookie")
    	orgs := GetOrgs(cookie)
    	if query == "screeps.*" || strings.HasPrefix(query, "screeps.*") {
    		acl := GetACL(orgs)
    		query = strings.Replace(query, "*", acl, 1)
    		vals := r.URL.Query()
    		vals.Set("query", query)
    		r.URL.RawQuery = vals.Encode()
    	}
    	next.ServeHTTP(w, r)
    }
    return http.HandlerFunc(ourFunc)
}

func Render(next http.Handler) http.Handler {
    ourFunc := func(w http.ResponseWriter, r *http.Request) {
    	cookie := r.Header.Get("Cookie")
    	orgs := GetOrgs(cookie)
    	acl := GetACL(orgs)
    	r.ParseForm()
    	targets := r.PostForm["target"]
    	validTargets := make([]string,0)
    	for _, target := range targets {
    		valid := false
    		for _, org := range orgs {
    			val := fmt.Sprintf("screeps.%s", org.Name)
    			if strings.Contains(target, val) {
    				valid = true
    				break
    			}
    		}
    		if !valid && strings.Contains(target, "screeps.*") {
    			target = strings.Replace(target, "screeps.*", fmt.Sprintf("screeps.%s", acl), 10)
    			valid = true
    		}
    		if valid {
    			validTargets = append(validTargets, target)
    		}    		
    	}
    	r.PostForm["target"] = validTargets
    	body := r.PostForm.Encode()
    	r.Body = ioutil.NopCloser(bytes.NewBufferString(body))
    	r.ContentLength = int64(len(body))
    	next.ServeHTTP(w, r)
    }
	return http.HandlerFunc(ourFunc)
}

func GetACL(orgs []GrafanaOrganization) string {
	var list []string
	for _, org := range orgs {
		list = append(list, org.Name)
	}
	s := strings.Join(list, ",")
	s = fmt.Sprintf("{%s}",s)
	return s
}

func GetOrgs(cookie string) []GrafanaOrganization {
	req, _ := http.NewRequest("GET", grafanaUrl + "/api/user/orgs", nil)
	req.Header.Add("Cookie", cookie)
	var client http.Client
	res, _ := client.Do(req)
	defer res.Body.Close()
	orgs := make([]GrafanaOrganization,0)
	err := json.NewDecoder(res.Body).Decode(&orgs)
	if err != nil {
		log.Fatal(err)
	}
	return orgs
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
	AllowChildren int `json:"allowChildren"`
	Expandable int `json:"expandable"`
	Id string `json:"id"`
	Leaf int `json:"leaf"`
	Text string `json:"text"`
}

type GrafanaOrganization struct {
	OrgId int
	Name string
	Role string
}