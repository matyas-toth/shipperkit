package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
	"shipper/internal/auth"
	"shipper/internal/cli"
	"shipper/internal/deploy"
	"shipper/internal/engine"
	"shipper/internal/events"
	"shipper/internal/httpapi"
	"shipper/internal/inventory"
	"shipper/internal/operations"
	"shipper/internal/store"
	"shipper/internal/ui"
)

var version = "1.0.0-rc.8" // x-release-please-version

func main() {
	if err := run(); err != nil {
		structured := false
		for _, arg := range os.Args[1:] {
			structured = structured || arg == "--json"
		}
		if structured {
			var apiError *cli.APIError
			if errors.As(err, &apiError) {
				json.NewEncoder(os.Stderr).Encode(map[string]any{"error": apiError})
			} else {
				json.NewEncoder(os.Stderr).Encode(map[string]any{"error": map[string]string{"code": "command_failed", "message": err.Error()}})
			}
		} else if len(os.Args) > 1 {
			slog.Error("Shipper command failed", "error", err)
		} else {
			slog.Error("Shipper stopped", "error", err)
		}
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
		case "connect", "apps", "inspect", "doctor", "plan", "deploy", "watch", "logs", "rollback", "task":
			return cli.Run(os.Args[1:], os.Stdout)
		case "bootstrap":
			return cli.Bootstrap(os.Args[2:], os.Stdout, os.Stderr)
		case "version":
			fmt.Println(version)
			return nil
		case "restore":
			flags := flag.NewFlagSet("restore", flag.ContinueOnError)
			archive := flags.String("archive", "", "Encrypted instance archive")
			identity := flags.String("identity", "", "Recovery key file")
			target := flags.String("data-dir", "", "Empty destination directory")
			if e := flags.Parse(os.Args[2:]); e != nil {
				return e
			}
			if *archive == "" || *identity == "" || *target == "" || flags.NArg() != 0 {
				return errors.New("usage: shipper restore --archive backup.age --identity recovery-key.txt --data-dir empty-directory")
			}
			if e := deploy.RecoverInstance(context.Background(), *archive, *identity, *target); e != nil {
				return e
			}
			fmt.Println("Instance recovered with applications stopped and schedules disabled. Review the installation before starting workloads.")
			return nil
		case "healthcheck":
			return healthcheck()
		case "help", "--help", "-h":
			fmt.Println("Usage: shipper [bootstrap|connect|apps|inspect|doctor|plan|deploy|watch|logs|rollback|task|version|healthcheck|claim|rotate-claim|reset-password|reset-factor|restore]\n\nConfiguration: SHIPPER_ADDR, SHIPPER_DATA_DIR, SHIPPER_PUBLIC_URL, DOCKER_HOST")
			return nil
		case "reset-password", "reset-factor", "claim", "rotate-claim", "update-ready", "checkpoint", "restore-checkpoint":
		default:
			return errors.New("usage: shipper [version|healthcheck|reset-password|restore]")
		}
	}
	dataDir := env("SHIPPER_DATA_DIR", ".shipper")
	if len(os.Args) > 1 && os.Args[1] == "restore-checkpoint" {
		id, err := checkpointFlag(os.Args[2:])
		if err != nil {
			return err
		}
		if os.Getenv("SHIPPER_MANAGED_INSTALL") != "1" {
			return errors.New("Update checkpoint recovery is reserved for the managed installer")
		}
		return store.RestoreCheckpoint(context.Background(), dataDir, id)
	}
	db, err := store.Open(dataDir)
	if err != nil {
		return err
	}
	defer db.Close()
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "reset-password":
			return resetPassword(db)
		case "reset-factor":
			return resetFactor(db)
		case "claim":
			return claimOwner(db, dataDir, os.Args[2:])
		case "update-ready":
			return db.UpdateReady(context.Background())
		case "checkpoint":
			id, err := checkpointFlag(os.Args[2:])
			if err != nil {
				return err
			}
			return db.WriteCheckpoint(context.Background(), id)
		case "rotate-claim":
			configured, err := db.Configured(context.Background())
			if err != nil {
				return err
			}
			if configured {
				return store.ErrAlreadyConfigured
			}
			if _, err = auth.RotateSetupClaim(dataDir); err != nil {
				return err
			}
			fmt.Println("A new setup token is available in the data directory for 30 minutes.")
			return nil
		}
	}
	recovered, e := deploy.RecoveredProxy(dataDir)
	if e != nil {
		return e
	}
	dashboardDomain, e := db.Setting(context.Background(), "dashboard_domain", "")
	if e != nil {
		return e
	}
	if dashboardDomain == "" {
		if raw, readErr := os.ReadFile(filepath.Join(dataDir, "dashboard-domain")); readErr == nil {
			domain, normalizeErr := deploy.NormalizeDashboardDomain(string(raw))
			if normalizeErr != nil {
				return normalizeErr
			}
			if domain != "" {
				if e = db.SaveSetting(context.Background(), "dashboard_domain", domain); e != nil {
					return e
				}
				dashboardDomain = domain
			}
			_ = os.Remove(filepath.Join(dataDir, "dashboard-domain"))
		}
	}
	publicURL := env("SHIPPER_PUBLIC_URL", recovered.DashboardURL)
	if dashboardDomain == "" && publicURL != "" {
		if u, parseErr := url.Parse(publicURL); parseErr == nil && u.Scheme == "https" && u.Hostname() != "" {
			if domain, normalizeErr := deploy.NormalizeDashboardDomain(u.Hostname()); normalizeErr == nil && domain != "" {
				if e = db.SaveSetting(context.Background(), "dashboard_domain", domain); e != nil {
					return e
				}
				dashboardDomain = domain
			}
		}
	}
	if dashboardDomain != "" {
		publicURL = "https://" + dashboardDomain
	}
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
	httpPort, err := strconv.Atoi(env("SHIPPER_HTTP_PORT", strconv.Itoa(func() int {
		if recovered.HTTPPort > 0 {
			return recovered.HTTPPort
		}
		return 8780
	}())))
	if err != nil || httpPort < 1 || httpPort > 65535 {
		return errors.New("SHIPPER_HTTP_PORT must be between 1 and 65535")
	}
	httpsPort, err := strconv.Atoi(env("SHIPPER_HTTPS_PORT", strconv.Itoa(func() int {
		if recovered.HTTPSPort > 0 {
			return recovered.HTTPSPort
		}
		return 8743
	}())))
	if err != nil || httpsPort < 1 || httpsPort > 65535 || httpPort == httpsPort {
		return errors.New("SHIPPER_HTTPS_PORT must be a distinct port between 1 and 65535")
	}
	proxyConfig := deploy.ProxyConfig{BindIP: env("SHIPPER_PROXY_BIND", func() string {
		if recovered.BindIP != "" {
			return recovered.BindIP
		}
		return "127.0.0.1"
	}()), HTTPPort: httpPort, HTTPSPort: httpsPort, ACMEEmail: env("SHIPPER_ACME_EMAIL", recovered.ACMEEmail), ACMECA: env("SHIPPER_ACME_CA", recovered.ACMECA)}
	proxyConfig.AutoHTTPS = os.Getenv("SHIPPER_AUTO_HTTPS") == "1" && httpPort == 80 && httpsPort == 443 && !net.ParseIP(proxyConfig.BindIP).IsLoopback()
	if proxyConfig.ACMEEmail != "" {
		address, e := mail.ParseAddress(proxyConfig.ACMEEmail)
		if e != nil || address.Address != proxyConfig.ACMEEmail {
			return errors.New("SHIPPER_ACME_EMAIL must be a valid email address")
		}
	}
	if net.ParseIP(proxyConfig.BindIP) == nil {
		return errors.New("SHIPPER_PROXY_BIND must be an IP address")
	}
	upstream := env("SHIPPER_DASHBOARD_UPSTREAM", recovered.DashboardUpstream)
	if os.Getenv("SHIPPER_MANAGED_INSTALL") == "1" && upstream == "" {
		upstream = "auto"
	}
	if upstream != "" {
		proxyConfig.DashboardURL = publicURL
		if upstream == "auto" {
			proxyConfig.DashboardContainer = os.Getenv("HOSTNAME")
			proxyConfig.DashboardUpstream = "http://dashboard-" + instance[:12] + ":8080"
		} else {
			proxyConfig.DashboardUpstream = upstream
		}
		if publicURL != "" {
			if err = deploy.ValidateDashboard(proxyConfig); err != nil {
				return err
			}
			dashboard, _ := url.Parse(publicURL)
			apps, e := db.Applications(context.Background())
			if e != nil {
				return e
			}
			for _, app := range apps {
				if app.Config.Domain == dashboard.Hostname() {
					return errors.New("Move the application's dashboard hostname before enabling dashboard HTTPS")
				}
				for _, domain := range app.Domains {
					if domain.Hostname == dashboard.Hostname() {
						return errors.New("Move the application's dashboard hostname before enabling dashboard HTTPS")
					}
				}
			}
		}
	}
	dockerRuntime := &deploy.DockerRuntime{Client: docker.Client(), Instance: instance, Config: proxyConfig, DataDir: dataDir, ACMEContact: db.ACMEContact, DashboardDomain: func(ctx context.Context) string {
		domain, _ := db.Setting(ctx, "dashboard_domain", "")
		return domain
	}}
	worker := deploy.NewWorker(db, dockerRuntime, hub, dataDir)
	worker.Refresh = monitor.Trigger
	operator := &operations.Service{DB: db, Docker: docker.Client(), Monitor: monitor, Instance: instance, DataDir: dataDir, DashboardURL: proxyConfig.DashboardURL, Publish: hub.Publish}
	worker.Operations = operator.Run
	worker.Maintenance = operator.ProcessMaintenance
	operator.RepairProxy = worker.RepairProxy
	proxyIdentity, _ := json.Marshal(proxyConfig)
	operator.ConfigIdentity = string(proxyIdentity)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	monitorDone := make(chan struct{})
	go func() { defer close(monitorDone); monitor.Run(ctx) }()
	workerDone := make(chan struct{})
	updateLock := filepath.Join(dataDir, "update.lock")
	managed := os.Getenv("SHIPPER_MANAGED_INSTALL") == "1"
	updating := func() bool {
		if !managed {
			return false
		}
		_, e := os.Stat(updateLock)
		return !errors.Is(e, os.ErrNotExist)
	}
	go func() {
		defer close(workerDone)
		for updating() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(250 * time.Millisecond):
			}
		}
		worker.Run(ctx)
	}()
	addr := env("SHIPPER_ADDR", "127.0.0.1:8080")
	apiConfig := httpapi.Config{PublicURL: publicURL, SetupToken: setupToken, Assets: ui.Handler(), Version: version, InstanceID: instance, Worker: worker, Proxy: proxyConfig, EffectiveProxy: dockerRuntime.EffectiveProxy, Operations: operator}
	if !configured && os.Getenv("SHIPPER_SETUP_TOKEN") == "" {
		apiConfig.SetupClaim = func() (string, error) { return auth.ReadSetupClaim(dataDir) }
	}
	handler := httpapi.New(db, monitor, hub, apiConfig)
	server := &http.Server{Addr: addr, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" && updating() {
			w.Header().Set("Retry-After", "5")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"code":"updating","message":"Shipper is being updated"}}`))
			return
		}
		handler.ServeHTTP(w, r)
	}), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
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

func checkpointFlag(args []string) (string, error) {
	flags := flag.NewFlagSet("checkpoint", flag.ContinueOnError)
	id := flags.String("id", "", "Checkpoint identity")
	if err := flags.Parse(args); err != nil {
		return "", err
	}
	if *id == "" || flags.NArg() != 0 {
		return "", errors.New("Provide --id CHECKPOINT")
	}
	return *id, nil
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
