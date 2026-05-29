// Command server is the reviews-api composition root: it wires the file stores,
// the iTunes feed fetcher, the service, the HTTP API, and a polling scheduler,
// then runs until SIGINT/SIGTERM and shuts down gracefully.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/souzavinny/reviews-api/internal/appstore"
	"github.com/souzavinny/reviews-api/internal/domain"
	"github.com/souzavinny/reviews-api/internal/httpapi"
	"github.com/souzavinny/reviews-api/internal/service"
	"github.com/souzavinny/reviews-api/internal/storage"
)

func main() {
	cfg := loadConfig()

	store, err := storage.NewFileStore(cfg.dataDir)
	if err != nil {
		log.Fatalf("init review store: %v", err)
	}
	registry, err := storage.NewAppStore(cfg.dataDir, cfg.seedApps)
	if err != nil {
		log.Fatalf("init app registry: %v", err)
	}
	svc := service.New(store, appstore.NewRSSFetcher(), registry)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Background initial poll triggered by an add. bgSem bounds how many run at
	// once and skips the poll when saturated (the scheduler catches it next
	// tick), so a burst of adds can't spawn unbounded goroutines. bgPolls is
	// tracked so shutdown can wait — OnAppAdded MUST be invoked synchronously
	// within the handler so every Add happens-before srv.Shutdown returns, and
	// thus before bgPolls.Wait() below.
	var bgPolls sync.WaitGroup
	bgSem := make(chan struct{}, cfg.concurrency)
	pollInBackground := func(appID string) {
		select {
		case bgSem <- struct{}{}:
		default:
			return
		}
		bgPolls.Add(1)
		go func() {
			defer bgPolls.Done()
			defer func() { <-bgSem }()
			if err := svc.PollApp(ctx, appID); err != nil {
				log.Printf("initial poll of app %s failed: %v", appID, err)
			}
		}()
	}

	srv := &http.Server{
		Addr: ":" + cfg.port,
		Handler: httpapi.New(svc, httpapi.Config{
			DefaultWindow: cfg.defaultWindow,
			AllowedOrigin: cfg.corsOrigin,
			OnAppAdded:    pollInBackground,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	var scheduler sync.WaitGroup
	scheduler.Add(1)
	go func() {
		defer scheduler.Done()
		runScheduler(ctx, svc, cfg.pollInterval, cfg.concurrency)
	}()

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("reviews-api listening on %s (poll every %s, default window %s)", srv.Addr, cfg.pollInterval, cfg.defaultWindow)
		serverErr <- srv.ListenAndServe()
	}()

	var exitErr error
	select {
	case <-ctx.Done():
		log.Println("shutting down")
	case err := <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server error: %v", err)
			exitErr = err
		}
	}
	stop() // cancel the shared context so the scheduler and background polls wind down

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
	scheduler.Wait()
	bgPolls.Wait()
	log.Println("stopped")

	if exitErr != nil {
		os.Exit(1)
	}
}

// runScheduler polls every registered app immediately, then on each tick, with
// bounded concurrency. It returns when ctx is cancelled, after in-flight polls.
func runScheduler(ctx context.Context, svc *service.Service, interval time.Duration, concurrency int) {
	sem := make(chan struct{}, concurrency)
	pollAll := func() {
		apps, err := svc.ListApps(ctx)
		if err != nil {
			log.Printf("scheduler: list apps: %v", err)
			return
		}
		var wg sync.WaitGroup
		for _, app := range apps {
			select {
			case <-ctx.Done():
				wg.Wait()
				return
			case sem <- struct{}{}:
			}
			wg.Add(1)
			go func(appID string) {
				defer wg.Done()
				defer func() { <-sem }()
				if err := svc.PollApp(ctx, appID); err != nil {
					log.Printf("scheduler: poll app %s: %v", appID, err)
				}
			}(app.ID)
		}
		wg.Wait()
	}

	pollAll()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pollAll()
		}
	}
}

type config struct {
	port          string
	dataDir       string
	seedApps      []domain.App
	pollInterval  time.Duration
	defaultWindow time.Duration
	corsOrigin    string
	concurrency   int
}

func loadConfig() config {
	cfg := config{
		port:          getenv("PORT", "8080"),
		dataDir:       getenv("DATA_DIR", "data"),
		pollInterval:  getDuration("POLL_INTERVAL", 5*time.Minute),
		defaultWindow: time.Duration(getInt("DEFAULT_WINDOW_HOURS", 48)) * time.Hour,
		corsOrigin:    getenv("CORS_ORIGIN", "*"),
		concurrency:   getInt("POLL_CONCURRENCY", 4),
	}
	if cfg.concurrency < 1 {
		cfg.concurrency = 1
	}
	for _, id := range strings.Split(getenv("APP_IDS", ""), ",") {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, err := strconv.ParseUint(id, 10, 64); err != nil {
			log.Printf("config: ignoring non-numeric APP_IDS entry %q", id)
			continue
		}
		cfg.seedApps = append(cfg.seedApps, domain.App{ID: id})
	}
	return cfg
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
