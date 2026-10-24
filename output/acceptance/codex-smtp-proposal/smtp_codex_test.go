package email

import (
	"context"
	"net"
	"testing"
	"time"
)

// Diagnostic reproducer: observes current behavior without blessing it as correct.
func TestCodexSMTPGreetingHonorsContext(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		time.Sleep(800 * time.Millisecond)
	}()
	sender, err := NewSMTPSender(Config{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port, FromAddress: "synthetic@example.test"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err = sender.SendEmail(ctx, "recipient@example.test", "Synthetic timeout probe", "", "synthetic")
	elapsed := time.Since(started)
	<-done
	if err == nil {
		t.Fatal("expected closed greeting connection error")
	}
	if elapsed > 500*time.Millisecond {
		t.Fatalf("SMTP ignored context: %s", elapsed)
	}
	t.Logf("FIX VERIFIED: 100ms context stopped SMTP greeting after %s", elapsed)
}
