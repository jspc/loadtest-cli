package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
)

var (
	client oauthClient
)

type oauthClient interface {
	Do(*http.Request) (*http.Response, error)
}

type queueResponse struct {
	Queued bool `json:"queued"`
}

type uploadResponse struct {
	Binary string `json:"binary"`
}

// QueueJob takes a HostBinary and uses it to queue a job
// against an agent
func QueueJob(hb HostBinary, job Job) (err error) {
	if hb.Host == "" {
		err = fmt.Errorf("Missing hostname")

		return
	}

	job.Binary = hb.Binary

	b, err := json.Marshal(job)
	if err != nil {
		return
	}

	body := bytes.NewBuffer(b)
	queuedResp := new(queueResponse)

	respBody, err := doRequest(setURL(hb.Host, "/queue"), body, queuedResp)
	if err != nil {
		return
	}

	if !queuedResp.Queued {
		err = fmt.Errorf("Could not queue job, received: %+v", string(respBody))
	}

	return
}

// UploadSchedule sends a schedule binary to an agent and returns
// a HostBinary of that host and binary combo
func UploadSchedule(f, addr string) (hb HostBinary, err error) {
	if addr == "" {
		err = fmt.Errorf("Missing hostname")

		return
	}

	hb.Host = addr

	r, err := os.Open(f)
	if err != nil {
		return
	}

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	fw, err := w.CreateFormFile("file", r.Name())
	if err != nil {
		return
	}

	if _, err = io.Copy(fw, r); err != nil {
		return
	}

	w.Close()

	req, err := http.NewRequest("POST", setURL(addr, "/upload"), &b)
	if err != nil {
		return
	}

	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := client.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status: %s", resp.Status)

		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	uploadResp := new(uploadResponse)
	err = json.Unmarshal(body, uploadResp)
	hb.Binary = uploadResp.Binary

	return
}

func doRequest(url string, b io.Reader, r interface{}) (respBody []byte, err error) {
	req, err := http.NewRequest("POST", url, b)
	if err != nil {
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()

	respBody, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("Received %q from %s, %s", resp.Status, url, string(respBody))

		return
	}

	err = json.Unmarshal(respBody, r)

	return
}

func setURL(addr, path string) string {
	u := new(url.URL)
	u.Host = fmt.Sprintf("%s:8081", addr)
	u.Path = path
	u.Scheme = "http"

	return u.String()
}
