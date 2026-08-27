package email

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"crypto/tls"
)

const providerSMTP = "smtp"

// smtpSender delivers mail through an SMTP server using the standard
// library. Gmail is the default backend (app password required).
type smtpSender struct {
	host     string
	port     int
	username string
	password string
	from     string
}

func newSMTPSender(cfg SMTPConfig) *smtpSender {
	return &smtpSender{
		host:     cfg.Host,
		port:     cfg.Port,
		username: cfg.Username,
		password: cfg.Password,
		from:     cfg.From,
	}
}

func (s *smtpSender) SendText(ctx context.Context, msg EmailMessage) error {
	return s.send(ctx, msg, false)
}

func (s *smtpSender) SendHTML(ctx context.Context, msg EmailMessage) error {
	return s.send(ctx, msg, true)
}

func (s *smtpSender) send(ctx context.Context, msg EmailMessage, withHTML bool) error {
	if err := validate(msg); err != nil {
		return err
	}
	from := resolveFrom(msg, s.from)
	raw, err := buildMessage(from, msg, withHTML && msg.HTML != "")
	if err != nil {
		return wrapError(ErrBadRequest, providerSMTP, err)
	}

	// Bcc recipients go on the envelope only, never in the headers.
	rcpts := append(append(append([]string{}, msg.To...), msg.Cc...), msg.Bcc...)

	if err := s.deliver(from, rcpts, raw); err != nil {
		return wrapError(classifySMTP(err), providerSMTP, err)
	}
	return nil
}

// deliver opens a TCP connection, enforces STARTTLS, authenticates, and
// submits the message. The context is not honored by net/smtp (no
// context-aware dial in stdlib); callers cancel at the HTTP layer / earlier.
// ponytail: if MIME multipart assembly or header encoding gets painful,
// swap net/smtp + mime for gopkg.in/mail.v2 (new dependency).
func (s *smtpSender) deliver(from string, to []string, raw []byte) error {
	addr := net.JoinHostPort(s.host, strconv.Itoa(s.port))

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	c, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return err
	}
	defer c.Quit()

	if ok, _ := c.Extension("STARTTLS"); !ok {
		return fmt.Errorf("email: SMTP server %s does not support STARTTLS", addr)
	}
	if err := c.StartTLS(&tls.Config{ServerName: s.host}); err != nil {
		return err
	}
	if s.username != "" {
		auth := smtp.PlainAuth("", s.username, s.password, s.host)
		if err := c.Auth(auth); err != nil {
			return err
		}
	}
	if err := c.Mail(from); err != nil {
		return err
	}
	for _, rcpt := range to {
		if err := c.Rcpt(rcpt); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(raw); err != nil {
		return err
	}
	return w.Close()
}

// buildMessage assembles the raw RFC 5322 message. When withHTML is false
// the body is text/plain; otherwise it is multipart/alternative.
func buildMessage(from string, msg EmailMessage, withHTML bool) ([]byte, error) {
	if from == "" {
		return nil, errors.New("email: From is required (set EmailMessage.From or provider default)")
	}

	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(msg.To, ", "))
	if len(msg.Cc) > 0 {
		fmt.Fprintf(&b, "Cc: %s\r\n", strings.Join(msg.Cc, ", "))
	}
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("UTF-8", msg.Subject))
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))

	if !withHTML {
		fmt.Fprintf(&b, "Content-Type: text/plain; charset=UTF-8\r\n")
		fmt.Fprintf(&b, "Content-Transfer-Encoding: 8bit\r\n\r\n")
		fmt.Fprintf(&b, "%s\r\n", msg.Text)
		return b.Bytes(), nil
	}

	boundary := randomBoundary()
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%s\r\n\r\n", boundary)

	fmt.Fprintf(&b, "--%s\r\n", boundary)
	fmt.Fprintf(&b, "Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	fmt.Fprintf(&b, "%s\r\n\r\n", msg.Text)

	fmt.Fprintf(&b, "--%s\r\n", boundary)
	fmt.Fprintf(&b, "Content-Type: text/html; charset=UTF-8\r\n\r\n")
	fmt.Fprintf(&b, "%s\r\n\r\n", msg.HTML)

	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return b.Bytes(), nil
}

func randomBoundary() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

// classifySMTP maps an SMTP transport error to a SendError sentinel.
// Gmail reports bad app passwords as 535 / 5.7.x, which are auth failures;
// everything else is ErrTransport.
func classifySMTP(err error) error {
	s := err.Error()
	if strings.HasPrefix(s, "535") || strings.Contains(s, "5.7") {
		return ErrAuth
	}
	return ErrTransport
}
