package factory

import (
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
)

func TestNewModuleLogger(t *testing.T) {
	logger := NewModuleLogger("subscriptions-controller")
	entry, ok := logger.(*logrus.Entry)
	if !ok {
		t.Fatalf("expected *logrus.Entry, got %T", logger)
	}
	if entry.Data["module"] != "subscriptions-controller" {
		t.Fatalf("expected module field, got %+v", entry.Data)
	}
}

func TestLoggerWithContext(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "rest-test-123")
	rec := httptest.NewRecorder()
	ctx := e.NewContext(req, rec)

	logger := LoggerWithContext(logrus.NewEntry(logrus.StandardLogger()), ctx)
	entry, ok := logger.(*logrus.Entry)
	if !ok {
		t.Fatalf("expected *logrus.Entry, got %T", logger)
	}
	if entry.Data["request_id"] != "rest-test-123" {
		t.Fatalf("expected request_id field, got %+v", entry.Data)
	}
}
