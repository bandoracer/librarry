package kindle

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net"
	"net/mail"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

const MaxFileBytes = 25 * 1024 * 1024
const sendTimeout = 60 * time.Second

type Message struct {
	ID, Title, Filename string
	Data                []byte
}
type Sender func(context.Context, Settings, Message) (state, detail string)

// sendSMTP never retries. After DATA, network failures may hide successful
// acceptance; only a definite SMTP rejection proves failure.
func sendSMTP(ctx context.Context, s Settings, m Message) (string, string) {
	return deliverSMTP(ctx, s, m, &tls.Config{ServerName: s.Host, MinVersion: tls.VersionTLS12})
}

func deliverSMTP(ctx context.Context, s Settings, m Message, tc *tls.Config) (string, string) {
	payload, err := encodeMessage(s, m)
	if err != nil {
		return "failed", "Unable to prepare email"
	}
	address := net.JoinHostPort(s.Host, fmt.Sprint(s.Port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return "failed", "SMTP connection failed"
	}
	defer conn.Close()
	deadline := time.Now().Add(sendTimeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	_ = conn.SetDeadline(deadline)
	rawConn := conn
	stop := context.AfterFunc(ctx, func() { _ = rawConn.Close() })
	defer stop()
	if s.TLSMode == "implicit" {
		secure := tls.Client(conn, tc)
		if secure.HandshakeContext(ctx) != nil {
			return "failed", "SMTP TLS handshake failed"
		}
		conn = secure
	}
	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		return "failed", "SMTP greeting failed"
	}
	defer c.Close()
	if s.TLSMode == "starttls" {
		if err = c.StartTLS(tc); err != nil {
			return "failed", "SMTP STARTTLS failed"
		}
	}
	if err = c.Auth(smtp.PlainAuth("", s.Username, s.Password, s.Host)); err != nil {
		return "failed", "SMTP authentication failed"
	}
	if err = c.Mail(s.From); err != nil {
		return "failed", "SMTP rejected the sender"
	}
	if err = c.Rcpt(s.Recipient); err != nil {
		return "failed", "SMTP rejected the recipient"
	}
	w, err := c.Data()
	if err != nil {
		return "failed", "SMTP refused the message"
	}
	if _, err = w.Write(payload); err != nil {
		return "unknown", "Connection lost during submission; check Kindle before sending again"
	}
	if err = w.Close(); err != nil {
		var rejection *textproto.Error
		if errors.As(err, &rejection) && rejection.Code >= 400 && rejection.Code <= 599 {
			return "failed", "SMTP rejected the message"
		}
		return "unknown", "SMTP acceptance could not be confirmed; check Kindle before sending again"
	}
	// A failed QUIT cannot undo the successful DATA acknowledgement.
	return "accepted", "Accepted by the email server; Kindle delivery is not confirmed"
}

func encodeMessage(s Settings, m Message) ([]byte, error) {
	if len(m.Data) > MaxFileBytes {
		return nil, errors.New("attachment too large")
	}
	if !plainAddress(s.From) || !kindleAddress(s.Recipient) || strings.ContainsAny(m.ID, "\r\n") {
		return nil, errors.New("invalid email headers")
	}
	var b bytes.Buffer
	parts := multipart.NewWriter(&b)
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: %s\r\nDate: %s\r\nMessage-ID: <%s@librarry.local>\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=%q\r\n\r\n", (&mail.Address{Name: s.FromName, Address: s.From}).String(), s.Recipient, mime.QEncoding.Encode("utf-8", strings.ReplaceAll(strings.ReplaceAll(m.Title, "\r", " "), "\n", " ")), time.Now().Format(time.RFC1123Z), m.ID, parts.Boundary())
	body, err := parts.CreatePart(textproto.MIMEHeader{"Content-Type": {"text/plain; charset=utf-8"}})
	if err != nil {
		return nil, err
	}
	_, _ = body.Write([]byte("Sent from Librarry.\r\n"))
	contentType := "application/epub+zip"
	if strings.HasSuffix(m.Filename, ".pdf") {
		contentType = "application/pdf"
	}
	if strings.HasSuffix(m.Filename, ".txt") {
		contentType = "text/plain"
	}
	attachment, err := parts.CreatePart(textproto.MIMEHeader{"Content-Type": {contentType}, "Content-Disposition": {mime.FormatMediaType("attachment", map[string]string{"filename": m.Filename})}, "Content-Transfer-Encoding": {"base64"}})
	if err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(m.Data)
	for len(encoded) > 0 {
		n := 76
		if len(encoded) < n {
			n = len(encoded)
		}
		fmt.Fprint(attachment, encoded[:n], "\r\n")
		encoded = encoded[n:]
	}
	if err = parts.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
