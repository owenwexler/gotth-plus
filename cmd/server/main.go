package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"

	"github.com/owenwexler/gotth-plus/internal/config"
	"github.com/owenwexler/gotth-plus/internal/csrf"
	"github.com/owenwexler/gotth-plus/internal/database"
	"github.com/owenwexler/gotth-plus/internal/factory"
	"github.com/owenwexler/gotth-plus/internal/handler"
	"github.com/owenwexler/gotth-plus/internal/middleware"
	"github.com/owenwexler/gotth-plus/internal/repository"
	"github.com/owenwexler/gotth-plus/internal/requestid"
	"github.com/owenwexler/gotth-plus/internal/seed"
	"github.com/owenwexler/gotth-plus/internal/service"
)

func main() {
	env := config.GetEnv()

	// JSON logs in production for log tooling, readable text in development; both carry request_id
	var logHandler slog.Handler = slog.NewTextHandler(os.Stderr, nil)
	if env.IsProduction() {
		logHandler = slog.NewJSONHandler(os.Stderr, nil)
	}
	slog.SetDefault(slog.New(requestid.NewLogHandler(logHandler)))

	port := fmt.Sprintf(":%s", env.Port)

	db, err := database.ConnectDb(env.DatabasePath)
	if err != nil {
		slog.Error("database connection failed", "err", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              port,
		Handler:           buildHandler(env, newMux(env, db)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	// SIGINT/SIGTERM (Ctrl+C, `docker stop`, air/templ restarts) cancel ctx and start a graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// bind first so "listening" is only logged once the port is really ours
	listener, err := net.Listen("tcp", srv.Addr)
	if err != nil {
		slog.Error("listen failed", "addr", srv.Addr, "err", err)
		closeDB(db)
		os.Exit(1)
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("server listening", "port", env.Port, "env", env.GoEnv)
		serveErr <- srv.Serve(listener)
	}()

	select {
	case err := <-serveErr:
		// Serve only returns on its own when it crashed; a graceful shutdown happens in the other case
		slog.Error("server stopped", "err", err)
		closeDB(db)
		os.Exit(1)
	case <-ctx.Done():
		stop() // a second signal now kills the process the default way instead of being swallowed
		slog.Info("shutting down, waiting for in-flight requests", "timeout", shutdownTimeout)
	}

	// Stop accepting connections and let in-flight requests finish, but not forever
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed, closing connections", "err", err)
		_ = srv.Close()
	}

	// only close the database once no handler can still be using it
	closeDB(db)
	slog.Info("server stopped")
}

// newMux wires the dependencies and registers every route. It lives outside main so the tests can
// serve the real routes.
func newMux(env config.Env, db *gorm.DB) *http.ServeMux {
	validate := validator.New()

	exampleRepo := repository.NewExampleRepository(db)
	greetingSvc := service.NewGreetingService()
	healthHandler := handler.NewHealthHandler(db)
	seedHandler := seed.NewSeedHandler(db, exampleRepo)

	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", staticFiles("static")))
	mux.HandleFunc("GET /healthz", healthHandler.Check)
	// {$} matches "/" only, so unknown paths 404 instead of rendering the home page
	mux.HandleFunc("GET /{$}", factory.RenderHome())

	// Example app: see "Removing the example app" in README.md
	mux.HandleFunc("POST /hello", factory.Hello(greetingSvc, validate))

	// Dev-only: the route is not registered at all in production, so it 404s.
	if !env.IsProduction() {
		mux.Handle("POST /seed", middleware.RequireAdminKey(env.SeedAdminKey,
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := seedHandler.Seed(r); err != nil {
					slog.ErrorContext(r.Context(), "seed failed", "err", err)
					http.Error(w, "seed failed", http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			})))
	}

	return mux
}

// staticFiles serves the files in dir, but never a directory listing.
func staticFiles(dir string) http.Handler {
	files := http.FileServer(http.Dir(dir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "" || strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
}

// closeDB releases the connection pool.
func closeDB(db *gorm.DB) {
	sqlDB, err := db.DB()
	if err != nil {
		slog.Error("getting database handle", "err", err)
		return
	}
	if err := sqlDB.Close(); err != nil {
		slog.Error("closing database", "err", err)
	}
}

// shutdownTimeout bounds how long in-flight requests get to finish; it matches the server's WriteTimeout.
const shutdownTimeout = 30 * time.Second

// maxBodyBytes caps every request body; raise it for routes that accept uploads.
const maxBodyBytes = 1 << 20

// buildHandler wraps the mux in the global middleware. Outermost runs first, so cheap rejections
// (rate limit, cross-origin, body size, CSRF) happen before any handler work.
func buildHandler(env config.Env, mux http.Handler) http.Handler {
	// Limits are per client IP. Development is far looser so test suites and live reload don't trip them.
	limits := middleware.RateLimits{
		All:       middleware.Limit{Rate: 20, Burst: 60},
		Mutations: middleware.Limit{Rate: 5, Burst: 20},
	}
	if !env.IsProduction() {
		limits = middleware.RateLimits{
			All:       middleware.Limit{Rate: 1000, Burst: 2000},
			Mutations: middleware.Limit{Rate: 500, Burst: 1000},
		}
	}

	// Rejects cross-origin browser requests using Sec-Fetch-Site/Origin (defence in depth next to the CSRF token).
	// Requests without those headers (curl, tests) pass; they are covered by the CSRF token check.
	crossOrigin := http.NewCrossOriginProtection()
	if !env.IsProduction() {
		// templ's live-reload proxy serves the app from another port
		for _, origin := range []string{"http://localhost:7331", "http://127.0.0.1:7331"} {
			if err := crossOrigin.AddTrustedOrigin(origin); err != nil {
				slog.Error("trusting dev origin", "origin", origin, "err", err)
			}
		}
	}

	handler := csrf.Middleware(env.IsProduction())(mux)
	handler = middleware.MaxBodyBytes(maxBodyBytes)(handler)
	handler = crossOrigin.Handler(handler)
	handler = middleware.RateLimit(limits)(handler)
	handler = middleware.Compress(handler)
	handler = middleware.SecurityHeaders(env.IsProduction())(handler)
	handler = requestid.Middleware(handler)

	return handler
}
