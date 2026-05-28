package mitm

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// Spin up the MITM TLS server on a random port; connect with our own root in
// the trust pool; verify the handshake succeeds and the default handler
// responds with the 501 stub envelope (proving the server signs a leaf for the
// SNI hostname on the fly).
func TestServer_TLSHandshakeUsingMintedLeaf(t *testing.T) {
	tmp := t.TempDir()
	cm, err := NewCertManager(tmp)
	if err != nil {
		t.Fatalf("cm: %v", err)
	}

	// Bind to an OS-chosen free port.
	probeLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := probeLn.Addr().String()
	_ = probeLn.Close()

	srv := NewServer(addr, cm, nil)
	go func() { _ = srv.Start() }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	})

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(cm.RootCAPEM())

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := tls.Dial("tcp", addr, &tls.Config{
			RootCAs:    pool,
			ServerName: "api.cursor.sh",
		})
		if err != nil {
			time.Sleep(75 * time.Millisecond)
			continue
		}
		_, _ = conn.Write([]byte("GET /probe HTTP/1.1\r\nHost: api.cursor.sh\r\nConnection: close\r\n\r\n"))
		b, _ := io.ReadAll(conn)
		conn.Close()
		text := string(b)
		if !strings.Contains(text, "HTTP/1.1") {
			t.Fatalf("unexpected response: %q", truncateForTest(text))
		}
		if !strings.Contains(text, "501") {
			t.Fatalf("expected 501 stub envelope, got: %q", truncateForTest(text))
		}
		if !strings.Contains(text, "api.cursor.sh") {
			t.Fatalf("response did not echo host: %q", truncateForTest(text))
		}
		return
	}
	t.Fatal("listener never became reachable within 5s")
}

func truncateForTest(s string) string {
	if len(s) > 200 {
		return s[:200] + "…"
	}
	return s
}
