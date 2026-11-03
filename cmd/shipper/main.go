package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/mail"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
	"shipper/internal/auth"
	"shipper/internal/deploy"
	"shipper/internal/engine"
	"shipper/internal/events"
	"shipper/internal/httpapi"
	"shipper/internal/inventory"
	"shipper/internal/store"
	"shipper/internal/ui"
)

var version = "0.6.0-dev"

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
	instance, err := db.Setting(context.Background(), "instance_id", auth.Token())
	if err != nil {
		return err
	}
	docker.Instance = instance
	httpPort, err := strconv.Atoi(env("SHIPPER_HTTP_PORT", "8780"))
	if err != nil || httpPort < 1 || httpPort > 65535 {
		return errors.New("SHIPPER_HTTP_PORT must be between 1 and 65535")
	}
	httpsPort, err := strconv.Atoi(env("SHIPPER_HTTPS_PORT", "8743"))
	if err != nil || httpsPort < 1 || httpsPort > 65535 || httpPort == httpsPort {
		return errors.New("SHIPPER_HTTPS_PORT must be a distinct port between 1 and 65535")
	}
	proxyConfig := deploy.ProxyConfig{BindIP: env("SHIPPER_PROXY_BIND", "127.0.0.1"), HTTPPort: httpPort, HTTPSPort: httpsPort, ACMEEmail: os.Getenv("SHIPPER_ACME_EMAIL"), ACMECA: os.Getenv("SHIPPER_ACME_CA")}
	if proxyConfig.ACMEEmail != "" {
		address, e := mail.ParseAddress(proxyConfig.ACMEEmail)
		if e != nil || address.Address != proxyConfig.ACMEEmail {
			return errors.New("SHIPPER_ACME_EMAIL must be a valid email address")
		}
	}
	if net.ParseIP(proxyConfig.BindIP) == nil {
		return errors.New("SHIPPER_PROXY_BIND must be an IP address")
	}
	worker := deploy.NewWorker(db, &deploy.DockerRuntime{Client: docker.Client(), Instance: instance, Config: proxyConfig, DataDir: dataDir}, hub, dataDir)
	worker.Refresh = monitor.Trigger
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	monitorDone := make(chan struct{})
	go func() { defer close(monitorDone); monitor.Run(ctx) }()
	workerDone := make(chan struct{})
	go func() { defer close(workerDone); worker.Run(ctx) }()
	addr := env("SHIPPER_ADDR", "127.0.0.1:8080")
	server := &http.Server{Addr: addr, Handler: httpapi.New(db, monitor, hub, httpapi.Config{PublicURL: publicURL, SetupToken: setupToken, Assets: ui.Handler(), Version: version, InstanceID: instance, Worker: worker, Proxy: proxyConfig}), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.ListenAndServe() }()
	slog.Info("Shipper is listening", "address", addr, "version", version)
	select {
	case err = <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			cancel()
			<-monitorDone
			<-workerDone
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
	<-workerDone
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
