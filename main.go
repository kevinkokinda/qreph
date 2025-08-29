package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mdp/qrterminal/v3"
)

type noteStore struct {
	content []byte
	once    sync.Once
	mu      sync.Mutex
}

func (s *noteStore) get() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	var content []byte
	s.once.Do(func() {
		content = s.content
		s.content = nil
	})
	return content
}

func getOutboundIP() (net.IP, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, errors.New("could not assert type to *net.UDPAddr")
	}

	return localAddr.IP, nil
}

func main() {
	log.SetFlags(0)

	stat, err := os.Stdin.Stat()
	if err != nil {
		log.Fatalf("failed to stat stdin: %v", err)
	}

	var content []byte
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		content, err = io.ReadAll(os.Stdin)
		if err != nil {
			log.Fatalf("failed to read from stdin: %v", err)
		}
	} else {
		if len(os.Args) < 2 {
			fmt.Println("usage: qreph <text> | <command> | qreph")
			return
		}
		content = []byte(strings.Join(os.Args[1:], " "))
	}

	if len(content) == 0 {
		log.Fatal("no content provided")
	}

	store := &noteStore{content: content}

	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		log.Fatalf("failed to generate random bytes: %v", err)
	}
	path := "/" + base64.URLEncoding.EncodeToString(randomBytes)

	done := make(chan struct{})

	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		note := store.get()
		if note == nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write(note)
		close(done)
	})

	server := &http.Server{
		Handler: mux,
	}

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatalf("failed to create listener: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server failed: %v", err)
		}
	}()

	ip, err := getOutboundIP()
	if err != nil {
		log.Fatalf("failed to get outbound ip: %v", err)
	}

	url := fmt.Sprintf("http://%s:%d%s", ip, port, path)

	fmt.Println("Serving note at:", url)
	qrterminal.Generate(url, qrterminal.L, os.Stdout)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-done:
	case <-stop:
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("server shutdown failed: %v", err)
	}
}
