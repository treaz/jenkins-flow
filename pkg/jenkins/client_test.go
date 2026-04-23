package jenkins

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/treaz/jenkins-flow/pkg/logger"
)

func TestWaitForBuild_ReturnsBuildNumber(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"building": false, "result": "SUCCESS", "number": 1234}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user:token", logger.New(logger.Error))
	result, number, err := c.WaitForBuild(context.Background(), srv.URL+"/")
	if err != nil {
		t.Fatalf("WaitForBuild failed: %v", err)
	}
	if result != "SUCCESS" {
		t.Errorf("expected SUCCESS, got %q", result)
	}
	if number != 1234 {
		t.Errorf("expected build number 1234, got %d", number)
	}
}
