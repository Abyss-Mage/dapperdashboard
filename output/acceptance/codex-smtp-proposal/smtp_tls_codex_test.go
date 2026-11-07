package email

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func TestCodexSMTPRejectsMissingSTARTTLS(t *testing.T) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	defer l.Close()
	commands := make(chan string, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		c, e := l.Accept()
		if e != nil {
			return
		}
		defer c.Close()
		c.SetDeadline(time.Now().Add(time.Second))
		fmt.Fprint(c, "220 synthetic SMTP\r\n")
		r := bufio.NewReader(c)
		for {
			line, e := r.ReadString('\n')
			if e != nil {
				return
			}
			commands <- line
			if strings.HasPrefix(line, "EHLO") {
				fmt.Fprint(c, "250-synthetic\r\n250 8BITMIME\r\n")
			} else {
				fmt.Fprint(c, "250 OK\r\n")
			}
		}
	}()
	s, e := NewSMTPSender(Config{Host: "127.0.0.1", Port: l.Addr().(*net.TCPAddr).Port, FromAddress: "sender@example.test"})
	if e != nil {
		t.Fatal(e)
	}
	_, e = s.SendEmail(context.Background(), "recipient@example.test", "Synthetic", "", "synthetic")
	if e == nil || !strings.Contains(e.Error(), "STARTTLS required") {
		t.Fatalf("expected required TLS rejection: %v", e)
	}
	<-done
	close(commands)
	for cmd := range commands {
		if strings.HasPrefix(cmd, "MAIL") || strings.HasPrefix(cmd, "RCPT") || strings.HasPrefix(cmd, "DATA") {
			t.Fatalf("sent envelope/body before TLS: %q", cmd)
		}
	}
}
