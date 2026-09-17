package kindle

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

func TestMIMEAttachmentRoundTrip(t *testing.T) {
	original := epub()
	raw, e := encodeMessage(validSettings(), Message{ID: "fixture", Title: "Book\r\nBcc: fake@example.org", Filename: "book.epub", Data: original})
	if e != nil {
		t.Fatal(e)
	}
	m, e := mail.ReadMessage(bytes.NewReader(raw))
	if e != nil {
		t.Fatal(e)
	}
	if m.Header.Get("Bcc") != "" {
		t.Fatal("header injected")
	}
	_, params, e := mime.ParseMediaType(m.Header.Get("Content-Type"))
	if e != nil {
		t.Fatal(e)
	}
	parts := multipart.NewReader(m.Body, params["boundary"])
	parts.NextPart()
	part, e := parts.NextPart()
	if e != nil {
		t.Fatal(e)
	}
	decoded, e := io.ReadAll(base64.NewDecoder(base64.StdEncoding, part))
	if e != nil || !bytes.Equal(decoded, original) {
		t.Fatal("attachment altered", e)
	}
}
func TestSMTPAcceptanceAndUncertainOutcomes(t *testing.T) {
	for _, mode := range []string{"implicit", "starttls"} {
		for _, outcome := range []string{"accepted", "rejected", "disconnect"} {
			t.Run(mode+"/"+outcome, func(t *testing.T) {
				certServer := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
				cert := certServer.TLS.Certificates[0]
				certServer.Close()
				pool := x509.NewCertPool()
				parsed, _ := x509.ParseCertificate(cert.Certificate[0])
				pool.AddCert(parsed)
				listener, e := net.Listen("tcp", "127.0.0.1:0")
				if e != nil {
					t.Fatal(e)
				}
				defer listener.Close()
				done := make(chan error, 1)
				go func() {
					conn, e := listener.Accept()
					if e != nil {
						done <- e
						return
					}
					defer conn.Close()
					conn.SetDeadline(time.Now().Add(5 * time.Second))
					serverTLS := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
					if mode == "implicit" {
						conn = tls.Server(conn, serverTLS)
					}
					reader := bufio.NewReader(conn)
					fmt.Fprint(conn, "220 fixture ready\r\n")
					for {
						line, e := reader.ReadString('\n')
						if e != nil {
							done <- nil
							return
						}
						switch {
						case strings.HasPrefix(line, "EHLO"):
							fmt.Fprint(conn, "250-fixture\r\n250-STARTTLS\r\n250 AUTH PLAIN\r\n")
						case strings.HasPrefix(line, "STARTTLS"):
							fmt.Fprint(conn, "220 Go ahead\r\n")
							conn = tls.Server(conn, serverTLS)
							reader = bufio.NewReader(conn)
						case strings.HasPrefix(line, "AUTH"):
							fmt.Fprint(conn, "235 authenticated\r\n")
						case strings.HasPrefix(line, "MAIL"), strings.HasPrefix(line, "RCPT"):
							fmt.Fprint(conn, "250 OK\r\n")
						case strings.HasPrefix(line, "DATA"):
							fmt.Fprint(conn, "354 send message\r\n")
							payload, e := textproto.NewReader(reader).ReadDotBytes()
							if e != nil {
								done <- e
								return
							}
							if !bytes.Contains(payload, []byte("Content-Disposition: attachment")) {
								done <- fmt.Errorf("missing attachment")
								return
							}
							if outcome == "disconnect" {
								done <- nil
								return
							}
							if outcome == "rejected" {
								fmt.Fprint(conn, "550 rejected\r\n")
							} else {
								fmt.Fprint(conn, "250 queued\r\n")
							}
						default:
							done <- fmt.Errorf("unexpected SMTP command")
							return
						}
					}
				}()
				s := validSettings()
				s.Host = "127.0.0.1"
				s.Port = listener.Addr().(*net.TCPAddr).Port
				s.TLSMode = mode
				state, _ := deliverSMTP(context.Background(), s, Message{ID: "fixture", Title: "Test", Filename: "book.epub", Data: epub()}, &tls.Config{RootCAs: pool, ServerName: "example.com", MinVersion: tls.VersionTLS12})
				expected := map[string]string{"accepted": "accepted", "rejected": "failed", "disconnect": "unknown"}[outcome]
				if state != expected {
					t.Fatalf("got %s want %s", state, expected)
				}
				if e := <-done; e != nil {
					t.Fatal(e)
				}
			})
		}
	}
}
