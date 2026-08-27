package email

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ── validate / resolveFrom ──────────────────────────────────────────────

func TestValidate(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  EmailMessage
		want string // error substring; empty = nil
	}{
		{"ok", EmailMessage{To: []string{"a@b.com"}, Text: "hi"}, ""},
		{"no To", EmailMessage{Text: "hi"}, "no To"},
		{"no Text", EmailMessage{To: []string{"a@b.com"}}, "Text body"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validate(tc.msg)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("validate() error = %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("validate() error = nil, want containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("validate() error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestResolveFrom(t *testing.T) {
	if got := resolveFrom(EmailMessage{From: "custom@test.com"}, "default@test.com"); got != "custom@test.com" {
		t.Errorf("resolveFrom with override = %q, want custom@test.com", got)
	}
	if got := resolveFrom(EmailMessage{}, "default@test.com"); got != "default@test.com" {
		t.Errorf("resolveFrom empty = %q, want default@test.com", got)
	}
}

// ── buildMessage (SMTP MIME) ───────────────────────────────────────────

func TestBuildMessageTextOnly(t *testing.T) {
	msg := EmailMessage{
		To:      []string{"a@b.com"},
		Subject: "Hello",
		Text:    "plain body",
	}
	raw, err := buildMessage("from@test.com", msg, false)
	if err != nil {
		t.Fatalf("buildMessage() error = %v", err)
	}
	s := string(raw)
	if !strings.Contains(s, "Content-Type: text/plain; charset=UTF-8") {
		t.Error("missing text/plain content type")
	}
	if !strings.Contains(s, "plain body") {
		t.Error("missing text body")
	}
	if strings.Contains(s, "multipart") {
		t.Error("text-only message must not be multipart")
	}
}

func TestBuildMessageMultipart(t *testing.T) {
	msg := EmailMessage{
		To:      []string{"a@b.com"},
		Subject: "Hello",
		Text:    "text part",
		HTML:    "<p>html part</p>",
	}
	raw, err := buildMessage("from@test.com", msg, true)
	if err != nil {
		t.Fatalf("buildMessage() error = %v", err)
	}
	s := string(raw)
	if !strings.Contains(s, "Content-Type: multipart/alternative") {
		t.Error("missing multipart content type")
	}
	if !strings.Contains(s, "text part") {
		t.Error("missing text part")
	}
	if !strings.Contains(s, "<p>html part</p>") {
		t.Error("missing html part")
	}
}

func TestBuildMessageCcBccHeaders(t *testing.T) {
	msg := EmailMessage{
		To:      []string{"to@test.com"},
		Cc:      []string{"cc@test.com"},
		Bcc:     []string{"bcc@test.com"},
		Subject: "test",
		Text:    "body",
	}
	raw, err := buildMessage("from@test.com", msg, false)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "Cc: cc@test.com") {
		t.Error("missing Cc header")
	}
	if strings.Contains(s, "bcc@test.com") {
		t.Error("Bcc must not appear in headers")
	}
}

func TestBuildMessageUnicodeSubject(t *testing.T) {
	msg := EmailMessage{
		To:      []string{"a@b.com"},
		Subject: "日本語テスト",
		Text:    "body",
	}
	raw, err := buildMessage("from@test.com", msg, false)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "Subject: =?UTF-8?q?") {
		t.Error("Subject must use Q-encoding for non-ASCII")
	}
}

func TestBuildMessageFromOverride(t *testing.T) {
	msg := EmailMessage{
		To:   []string{"a@b.com"},
		From: "override@test.com",
		Text: "body",
	}
	// resolveFrom already tested separately — buildMessage uses the resolved value directly.
	raw, err := buildMessage("override@test.com", msg, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "From: override@test.com") {
		t.Error("From override not applied")
	}
}

// ── classifyHTTPStatus ─────────────────────────────────────────────────

func TestClassifyHTTPStatus(t *testing.T) {
	for _, tc := range []struct {
		code int
		want error
	}{
		{401, ErrAuth},
		{403, ErrAuth},
		{429, ErrRateLimited},
		{400, ErrBadRequest},
		{500, ErrTransport},
		{503, ErrTransport},
	} {
		if got := classifyHTTPStatus(tc.code); got != tc.want {
			t.Errorf("classifyHTTPStatus(%d) = %v, want %v", tc.code, got, tc.want)
		}
	}
}

// ── REST provider integration (httptest) ───────────────────────────────

func TestResendProviderRequestShape(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/emails" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if auth := r.Header.Get("Authorization"); auth != "Bearer test-key" {
			t.Errorf("Authorization = %q, want Bearer test-key", auth)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	s := &resendSender{
		apiKey:  "test-key",
		from:    "from@test.com",
		client:  srv.Client(),
		baseURL: srv.URL,
	}

	err := s.SendText(context.Background(), EmailMessage{
		To:      []string{"a@b.com"},
		Subject: "sub",
		Text:    "body",
	})
	if err != nil {
		t.Fatalf("SendText() error = %v", err)
	}
	if got["from"] != "from@test.com" {
		t.Errorf("from = %v, want from@test.com", got["from"])
	}
}

func TestBrevoProviderRequestShape(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/smtp/email" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if key := r.Header.Get("api-key"); key != "test-key" {
			t.Errorf("api-key = %q, want test-key", key)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := &brevoSender{
		apiKey:  "test-key",
		from:    "from@test.com",
		client:  srv.Client(),
		baseURL: srv.URL,
	}

	err := s.SendText(context.Background(), EmailMessage{
		To:      []string{"a@b.com"},
		Subject: "sub",
		Text:    "body",
	})
	if err != nil {
		t.Fatalf("SendText() error = %v", err)
	}
	sender, _ := got["sender"].(map[string]any)
	if sender["email"] != "from@test.com" {
		t.Errorf("sender.email = %v, want from@test.com", sender["email"])
	}
}

func TestRESTAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	defer srv.Close()

	s := &resendSender{
		apiKey:  "bad-key",
		from:    "from@test.com",
		client:  srv.Client(),
		baseURL: srv.URL,
	}

	err := s.SendText(context.Background(), EmailMessage{
		To:   []string{"a@b.com"},
		Text: "body",
	})
	var se *SendError
	if !errors.As(err, &se) {
		t.Fatalf("error = %T, want *SendError", err)
	}
	if se.Code != ErrAuth {
		t.Errorf("Code = %v, want ErrAuth", se.Code)
	}
	if se.Provider != "resend" {
		t.Errorf("Provider = %q, want resend", se.Provider)
	}
}

func TestRESTRateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte("rate limited"))
	}))
	defer srv.Close()

	s := &brevoSender{
		apiKey:  "test-key",
		from:    "from@test.com",
		client:  srv.Client(),
		baseURL: srv.URL,
	}

	err := s.SendText(context.Background(), EmailMessage{
		To:   []string{"a@b.com"},
		Text: "body",
	})
	var se *SendError
	if !errors.As(err, &se) || se.Code != ErrRateLimited {
		t.Fatalf("error = %v, want ErrRateLimited", err)
	}
}

func TestRESTTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer srv.Close()

	s := &resendSender{
		apiKey:  "test-key",
		from:    "from@test.com",
		client:  srv.Client(),
		baseURL: srv.URL,
	}

	err := s.SendText(context.Background(), EmailMessage{
		To:   []string{"a@b.com"},
		Text: "body",
	})
	var se *SendError
	if !errors.As(err, &se) || se.Code != ErrTransport {
		t.Fatalf("error = %v, want ErrTransport", err)
	}
}

// ── factory ──────────────────────────────────────────────────────────

func TestNewEmailSenderUnknownProvider(t *testing.T) {
	_, err := NewEmailSender(Config{Provider: "ses"})
	if err == nil {
		t.Fatal("NewEmailSender() error = nil, want error for unknown provider")
	}
}

func TestNewEmailSenderSMTP(t *testing.T) {
	s, err := NewEmailSender(Config{
		Provider: ProviderSMTP,
		SMTP:     SMTPConfig{Host: "localhost", Port: 25, Username: "u", Password: "p", From: "f@e.com"},
	})
	if err != nil {
		t.Fatalf("NewEmailSender() error = %v", err)
	}
	if _, ok := s.(*smtpSender); !ok {
		t.Errorf("got %T, want *smtpSender", s)
	}
}

func TestNewEmailSenderDisabledIsNoOp(t *testing.T) {
	s, err := NewEmailSender(Config{Provider: ""})
	if err != nil {
		t.Fatalf("NewEmailSender() error = %v", err)
	}
	if err := s.SendText(context.Background(), EmailMessage{To: []string{"a@b.com"}, Text: "x"}); err != nil {
		t.Errorf("noop SendText() error = %v", err)
	}
	if err := s.SendHTML(context.Background(), EmailMessage{To: []string{"a@b.com"}, Text: "x", HTML: "<p>hi</p>"}); err != nil {
		t.Errorf("noop SendHTML() error = %v", err)
	}
}

// ── header injection ──────────────────────────────────────────────────

func TestValidateRejectsHeaderInjection(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  EmailMessage
	}{
		{"subject CRLF", EmailMessage{To: []string{"a@b.com"}, Subject: "sub\r\ninj", Text: "body"}},
		{"subject LF", EmailMessage{To: []string{"a@b.com"}, Subject: "sub\ninj", Text: "body"}},
		{"subject CR", EmailMessage{To: []string{"a@b.com"}, Subject: "sub\rinj", Text: "body"}},
		{"to CRLF", EmailMessage{To: []string{"a@b.com\r\nX-Injected: true"}, Text: "body"}},
		{"cc CRLF", EmailMessage{To: []string{"a@b.com"}, Cc: []string{"c@b.com\r\nX-Injected: true"}, Text: "body"}},
		{"from CRLF", EmailMessage{To: []string{"a@b.com"}, From: "f\r\nX-Injected: true", Text: "body"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validate(tc.msg)
			var se *SendError
			if !errors.As(err, &se) || se.Code != ErrBadRequest {
				t.Fatalf("validate() error = %v, want ErrBadRequest", err)
			}
		})
	}
}

// ── context timeout (SMTP dial) ──────────────────────────────────────

func TestSMTPDeliverRespectsContextDeadline(t *testing.T) {
	// Server accepts connections but never sends the SMTP greeting, so the
	// client blocks and the context deadline must fire.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				time.Sleep(5 * time.Second)
			}(c)
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	s := &smtpSender{host: "127.0.0.1", port: port, from: "f@e.com"}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err = s.SendText(ctx, EmailMessage{To: []string{"a@b.com"}, Text: "body"})
	var se *SendError
	if !errors.As(err, &se) {
		t.Fatalf("error = %T %v, want *SendError", err, err)
	}
	if se.Code != ErrTimeout {
		t.Errorf("Code = %v, want ErrTimeout (got %v)", se.Code, se)
	}
}
