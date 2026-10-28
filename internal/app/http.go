package app

import (
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/BIGGASSS/FlClash-MyGo/internal/store"
)

// Capture preferences on the UI thread; a worker must not read live settings.
func (a *App) httpClient() *http.Client {
	return requestClient(a.cfg, a.status == "running" && !a.safe)
}

func requestClient(settings store.Settings, running bool) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	if running {
		proxy := &url.URL{Scheme: "http", Host: net.JoinHostPort("localhost", strconv.Itoa(store.Int(settings.Overrides["mixed-port"])))}
		credential := ""
		switch values := settings.Overrides["authentication"].(type) {
		case []any:
			if len(values) > 0 {
				credential = store.String(values[0])
			}
		case []string:
			if len(values) > 0 {
				credential = values[0]
			}
		}
		if credential != "" {
			user, password, hasPassword := strings.Cut(credential, ":")
			if hasPassword {
				proxy.User = url.UserPassword(user, password)
			} else {
				proxy.User = url.User(user)
			}
		}
		transport.Proxy = func(request *http.Request) (*url.URL, error) {
			if strings.EqualFold(request.URL.Hostname(), "localhost") {
				return nil, nil
			}
			return proxy, nil
		}
	}
	transport.TLSClientConfig = &tls.Config{
		// This is the explicit imported/user certificate preference. New and
		// existing native settings verify certificates unless it is selected.
		InsecureSkipVerify: settings.SkipCertificateVerify,
	}
	return &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(_ *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return errors.New("stopped after 5 redirects")
		}
		return nil
	}}
}
