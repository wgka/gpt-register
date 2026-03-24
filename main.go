package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"codex-register/internal/config"
	"codex-register/internal/server"
	"codex-register/internal/telegrambot"
)

func main() {
	cfg := config.Load()
	app := server.NewApp(cfg)
	handler := server.NewRouterWithAPI(cfg, app.Handler)

	listener, addr := listenWithFallback(cfg)
	log.Printf("listening on http://%s", addr)

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app.StartBackground(rootCtx)

	go func() {
		if err := telegrambot.Start(rootCtx, app.Tasks); err != nil && err != context.Canceled {
			log.Printf("telegram-bot: %v", err)
		}
	}()

	if err := http.Serve(listener, handler); err != nil {
		log.Fatal(err)
	}
}

func listenWithFallback(cfg config.Settings) (net.Listener, string) {
	if cfg.PortProvided() {
		listener, err := net.Listen("tcp", cfg.Addr())
		if err != nil {
			log.Fatal(err)
		}
		return listener, listener.Addr().String()
	}

	startPort, err := strconv.Atoi(cfg.WebUIPort)
	if err != nil {
		log.Fatalf("invalid port %q: %v", cfg.WebUIPort, err)
	}

	for port := startPort; port < startPort+20; port++ {
		addr := net.JoinHostPort(cfg.WebUIHost, strconv.Itoa(port))
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			if port != startPort {
				log.Printf("default port %d is busy, switched to %d", startPort, port)
			}
			return listener, addr
		}
	}

	log.Fatalf("no available port found in range %d-%d", startPort, startPort+19)
	return nil, ""
}
