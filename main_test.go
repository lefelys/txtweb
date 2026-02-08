package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeResolver struct {
	records map[string][]string
	errs    map[string]error
}

func (f *fakeResolver) LookupTXT(_ context.Context, host string) ([]string, error) {
	if err, ok := f.errs[host]; ok {
		return nil, err
	}
	if records, ok := f.records[host]; ok {
		return append([]string(nil), records...), nil
	}
	return nil, nil
}

func TestRunWithConfiguresServer(t *testing.T) {
	var captured *http.Server

	err := runWith(&fakeResolver{}, func(server *http.Server) error {
		captured = server
		return http.ErrServerClosed
	})
	if err != nil {
		t.Fatalf("runWith returned error: %v", err)
	}
	if captured == nil {
		t.Fatal("expected server to be passed to serve function")
	}
	if captured.Addr != ":80" {
		t.Fatalf("expected addr :80, got %q", captured.Addr)
	}
	if captured.ReadHeaderTimeout != 5*time.Second {
		t.Fatalf("expected ReadHeaderTimeout 5s, got %v", captured.ReadHeaderTimeout)
	}
	if captured.Handler == nil {
		t.Fatal("expected non-nil server handler")
	}
}

func TestRunWithReturnsServeError(t *testing.T) {
	expectedErr := errors.New("serve failed")

	err := runWith(&fakeResolver{}, func(_ *http.Server) error {
		return expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
}

func TestHandlerReturns404WhenTXTRecordMissing(t *testing.T) {
	handler := newHandler(&fakeResolver{})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
	if rec.Header().Get(poweredByHeaderName) != poweredByHeaderValue {
		t.Fatalf("expected %s header to be %q, got %q", poweredByHeaderName, poweredByHeaderValue, rec.Header().Get(poweredByHeaderName))
	}
	if !strings.Contains(rec.Body.String(), "txtweb") {
		t.Fatalf("expected index header body, got %q", rec.Body.String())
	}
}

func TestHandlerReturnsPlainTextResponse(t *testing.T) {
	handler := newHandler(&fakeResolver{
		records: map[string][]string{
			"_txtweb.example.com": {"hello", "world"},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("content-type"); got != defaultPlainContentType {
		t.Fatalf("expected content-type %q, got %q", defaultPlainContentType, got)
	}
	if got := rec.Body.String(); got != "hello\nworld" {
		t.Fatalf("expected body %q, got %q", "hello\nworld", got)
	}
	if rec.Header().Get(poweredByHeaderName) != poweredByHeaderValue {
		t.Fatalf("expected %s header to be %q, got %q", poweredByHeaderName, poweredByHeaderValue, rec.Header().Get(poweredByHeaderName))
	}
}

func TestHandlerReturnsWrappedHTMLWithoutEscapingTags(t *testing.T) {
	handler := newHandler(&fakeResolver{
		records: map[string][]string{
			"_txtweb.example.com":     {"<h1>hi</h1>"},
			"_txtweb_cfg.example.com": {"html-wrap=true;html-align=top-right;html-max-width=600px;html-bg=red;html-fg=white"},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("content-type"); got != defaultWrappedContentType {
		t.Fatalf("expected content-type %q, got %q", defaultWrappedContentType, got)
	}

	body := rec.Body.String()
	for _, want := range []string{
		"<!--",
		"<!doctype html>",
		"<h1>hi</h1>",
		"justify-content:flex-end",
		"text-align:left",
		"max-width:600px",
		"background:red",
		"color:white",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected response body to contain %q, got %q", want, body)
		}
	}
	if strings.Contains(body, "&lt;h1&gt;") {
		t.Fatalf("expected raw HTML tags, got escaped output: %q", body)
	}
}

func TestHandlerReturnsClownEmojiFromEscapedText(t *testing.T) {
	clownEmoji := "\360\237\244\241"

	handler := newHandler(&fakeResolver{
		records: map[string][]string{
			"_txtweb.example.com": {"A! " + clownEmoji},
		},
	})
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if got := rec.Header().Get("content-type"); got != defaultPlainContentType {
		t.Fatalf("expected content-type %q, got %q", defaultPlainContentType, got)
	}
	if got := rec.Body.String(); got != "A! 🤡" {
		t.Fatalf("expected clown emoji output %q, got %q", "A! 🤡", got)
	}
}
