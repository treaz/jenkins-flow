package logger

import (
	"bytes"
	"io"
	"net/http"
	"strings"
)

// LoggingRoundTripper logs HTTP requests and responses
type LoggingRoundTripper struct {
	Wrapped http.RoundTripper
	Logger  *Logger
}

func (l *LoggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	currentLevel := l.Logger.GetLevel()

	// Only log request if level is DEBUG or TRACE
	if currentLevel >= Debug {
		l.Logger.Debugf("HTTP Request: %s %s", req.Method, req.URL)
	}

	if currentLevel >= Trace {
		l.dumpRequest(req)
	}

	resp, err := l.Wrapped.RoundTrip(req)
	if err != nil {
		l.Logger.Errorf("HTTP Error: %v", err)
		return nil, err
	}

	// Only log response if level is DEBUG or TRACE
	if currentLevel >= Debug {
		l.Logger.Debugf("HTTP Response: %s %s -> %s", req.Method, req.URL, resp.Status)
	}

	if currentLevel >= Trace {
		l.dumpResponse(resp)
	}

	return resp, nil
}

func (l *LoggingRoundTripper) dumpRequest(req *http.Request) {
	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(body)) // Restore body
	}

	l.Logger.Tracef("--- Request Headers ---")
	for k, v := range req.Header {
		if strings.EqualFold(k, "Authorization") {
			l.Logger.Tracef("%s: [REDACTED]", k)
		} else {
			l.Logger.Tracef("%s: %s", k, strings.Join(v, ", "))
		}
	}

	if len(body) > 0 {
		l.Logger.Tracef("--- Request Body ---")
		l.Logger.Tracef("%s", string(body))
	}
}

func (l *LoggingRoundTripper) dumpResponse(resp *http.Response) {
	var body []byte
	if resp.Body != nil {
		body, _ = io.ReadAll(resp.Body)
		resp.Body = io.NopCloser(bytes.NewBuffer(body)) // Restore body
	}

	l.Logger.Tracef("--- Response Headers ---")
	for k, v := range resp.Header {
		l.Logger.Tracef("%s: %s", k, strings.Join(v, ", "))
	}

	if len(body) > 0 {
		l.Logger.Tracef("--- Response Body ---")
		l.Logger.Tracef("%s", string(body))
	}
}
