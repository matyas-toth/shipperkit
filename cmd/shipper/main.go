package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
	"shipper/internal/auth"
	"shipper/internal/engine"
	"shipper/internal/events"
	"shipper/internal/httpapi"
	"shipper/internal/inventory"
	"shipper/internal/store"
	"shipper/internal/ui"
)

var version = "0.1.0-dev"

func main() {
	if err := run(); err != nil {
		slog.Error("Shipper stopped", "error", err)
		os.Exit(1)
	}
}
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
func run() error {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			fmt.Println(version)
			return nil
		case "healthcheck":
			return healthcheck()
		case "help", "--help", "-h":
			fmt.Println("Usage: shipper [version|healthcheck|reset-password]\n\nConfiguration: SHIPPER_ADDR, SHIPPER_DATA_DIR, SHIPPER_PUBLIC_URL, DOCKER_HOST")
			return nil
		case "reset-password":
		default:
			return errors.New("usage: shipper [version|healthcheck|reset-password]")
		}
	}
	dataDir := env("SHIPPER_DATA_DIR", ".shipper")
	db, err := store.Open(dataDir)
	if err != nil {
		return err
	}
	defer db.Close()
	if len(os.Args) > 1 {
		return resetPassword(db)
	}
	publicURL := os.Getenv("SHIPPER_PUBLIC_URL")
	if err = httpapi.ValidatePublicURL(publicURL); err != nil {
		return err
	}
	configured, err := db.Configured(context.Background())
	if err != nil {
		return err
	}
	setupToken := ""
	if !configured {
		setupToken, err = auth.SetupToken(dataDir, os.Getenv("SHIPPER_SETUP_TOKEN"))
		if err != nil {
			return err
		}
		if os.Getenv("SHIPPER_SETUP_TOKEN") == "" {
			slog.Info("First administrator setup is ready", "token_file", filepath.Join(dataDir, "setup-token"))
		}
	}
	docker, err := engine.New()
	if err != nil {
		return err
	}
	defer docker.Close()
	hub := events.New()
	monitor := inventory.New(docker, db, hub)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	monitorDone := make(chan struct{})
	go func() { defer close(monitorDone); monitor.Run(ctx) }()
	addr := env("SHIPPER_ADDR", "127.0.0.1:8080")
	server := &http.Server{Addr: addr, Handler: httpapi.New(db, monitor, hub, httpapi.Config{PublicURL: publicURL, SetupToken: setupToken, Assets: ui.Handler(), Version: version}), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	slog.Info("Shipper is listening", "address", addr, "version", version)
	select {
	case err = <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			cancel()
			<-monitorDone
			return err
		}
	case <-ctx.Done():
	}
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	// Close SSE connections as well as ordinary keep-alive requests on shutdown.
	server.Close()
	_ = server.Shutdown(shutdownCtx)
	<-monitorDone
	return nil
}

func healthcheck() error {
	_, port, err := net.SplitHostPort(env("SHIPPER_ADDR", "127.0.0.1:8080"))
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get("http://" + net.JoinHostPort("127.0.0.1", port) + "/api/health")
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck: HTTP %d", response.StatusCode)
	}
	return nil
}
func resetPassword(db *store.Store) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("password recovery requires an interactive terminal")
	}
	fmt.Print("Administrator email: ")
	reader := bufio.NewReader(os.Stdin)
	email, err := reader.ReadString('\n')
	if err != nil {
		return err
	}
	fmt.Print("New password: ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return err
	}
	hash, err := auth.Password(string(password))
	if err != nil {
		return err
	}
	if err = db.ResetPassword(context.Background(), strings.TrimSpace(email), hash); err != nil {
		return err
	}
	fmt.Println("Password updated and all sessions revoked.")
	return nil
}
