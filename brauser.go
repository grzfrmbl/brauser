package brauser

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"
)

// The brauser package is intended as a preconfigured lightweight http client
// based on the go http.client with some useful defaults such as a cookiejar,
// sane timeouts, a bit of logging and request retries.

type Options struct {
	Timeout             time.Duration
	TlsHandshakeTimeout time.Duration
	DialTimeout         time.Duration
	Tries               int
	Verbose             bool
}

type WebClient struct {
	cl          *http.Client
	options     Options
	lastTimeout time.Time
}

func CreateWebClient(opts ...Options) WebClient {
	jar, _ := cookiejar.New(nil)

	o := Options{}
	if len(opts) != 1 {
		// Default
		o = Options{
			Timeout:             time.Second * 60,
			TlsHandshakeTimeout: 5 * time.Second,
			DialTimeout:         5 * time.Second,
			Tries:               1,
			Verbose:             false,
		}
	} else {
		// User defined
		o = opts[0]
	}

	var netTransport = &http.Transport{
		Dial:                (&net.Dialer{Timeout: o.DialTimeout}).Dial,
		TLSHandshakeTimeout: o.TlsHandshakeTimeout,
	}

	return WebClient{
		cl: &http.Client{
			Jar:       jar,
			Timeout:   o.Timeout,
			Transport: netTransport,
		},
		options: o,
	}

}

func (w *WebClient) Get(path string, params map[string]string) (data []byte, err error) {
	return w.fetch("GET", path, params, nil)
}
func (w *WebClient) Post(path string, params map[string]string, payload io.Reader) (data []byte, err error) {
	return w.fetch("POST", path, params, payload)
}
func (w *WebClient) CustomRequest(method, path string, params map[string]string, payload io.Reader) (data []byte, err error) {
	return w.fetch(method, path, params, payload)
}
func (w *WebClient) ExportCookies(file, site string) error {
	u, err := url.Parse(site)
	if err != nil {
		return err
	}
	data, err := json.Marshal(w.cl.Jar.Cookies(u))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(file, data, 0644)
	if err != nil {
		return err
	}
	return nil
}
func (w *WebClient) ImportCookies(file, site string) error {
	u, err := url.Parse(site)
	if err != nil {
		return err
	}
	d, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	var cookies []*http.Cookie

	err = json.Unmarshal(d, &cookies)
	if err != nil {
		return err
	}

	w.cl.Jar.SetCookies(u, cookies)

	return nil
}
func (w *WebClient) fetch(method, path string, params map[string]string, payload io.Reader) (data []byte, err error) {
	req, err := http.NewRequest(method, path, payload)
	if err != nil {
		return
	}

	for k, p := range params {
		req.Header.Add(k, p)
	}

	w.logFetch(req.Method, "  ", req.URL.String(), "  ")

	tryCount := 0
retry:

	resp, err := w.cl.Do(req)

	if err != nil {
		// Call failed, try again as specified in retries
		if tryCount < w.options.Tries {
			w.logFetch("retry after", w.options.Timeout, "due to call failure,", err)
			time.Sleep(w.options.Timeout)

			tryCount++
			goto retry
		} else {
			w.logFetch("aborting fetch")
		}

		return
	}
	defer resp.Body.Close()
	w.logFetch(resp.StatusCode)

	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	return
}

func (w *WebClient) logFetch(s ...interface{}) {
	if w.options.Verbose {
		fmt.Println(s)
	}
}
