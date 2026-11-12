package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ATLURI0001/target-reconciliation-service/controllers"
	"github.com/ATLURI0001/target-reconciliation-service/internal/core"
	"github.com/ATLURI0001/target-reconciliation-service/internal/httpapi"
	"github.com/ATLURI0001/target-reconciliation-service/internal/store"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func validateEnvironment() error {
	if os.Getenv("DATABASE_URL") == "" {
		return errors.New("DATABASE_URL must explicitly select a dedicated database")
	}
	operator, viewer := os.Getenv("TRS_OPERATOR_TOKEN"), os.Getenv("TRS_VIEWER_TOKEN")
	if len(operator) < 32 || len(viewer) < 32 || operator == viewer {
		return errors.New("distinct operator and viewer tokens of at least 32 bytes required")
	}
	if k := env("TRS_KUBERNETES", "false"); k != "true" && k != "false" {
		return errors.New("TRS_KUBERNETES must be true or false")
	}
	if env("TRS_KUBERNETES", "false") == "true" && env("TRS_NAMESPACE", "trs-demo") != "trs-demo" {
		return errors.New("initial controller only supports trs-demo")
	}
	return nil
}
func run() error {
	if err := validateEnvironment(); err != nil {
		return err
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	db, err := store.NewPostgres(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return errors.New("database initialization failed (check DATABASE_URL and connectivity)")
	}
	defer db.Close()
	svc := core.NewService(db)
	api, err := httpapi.New(svc, os.Getenv("TRS_OPERATOR_TOKEN"), os.Getenv("TRS_VIEWER_TOKEN"))
	if err != nil {
		return err
	}
	failures := make(chan error, 2)
	if env("TRS_KUBERNETES", "false") == "true" {
		cfg, err := ctrl.GetConfig()
		if err != nil {
			return errors.New("Kubernetes configuration unavailable")
		}
		scheme := runtime.NewScheme()
		if err = clientgoscheme.AddToScheme(scheme); err != nil {
			return err
		}
		namespace := env("TRS_NAMESPACE", "trs-demo")
		if namespace != "trs-demo" {
			return errors.New("initial controller only supports trs-demo")
		}
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{Scheme: scheme, Cache: cache.Options{DefaultNamespaces: map[string]cache.Config{namespace: {}}}, LeaderElection: true, LeaderElectionNamespace: namespace, LeaderElectionID: "target-reconciliation-service-controller", Metrics: server.Options{BindAddress: "0"}, HealthProbeBindAddress: "0"})
		if err != nil {
			return err
		}
		cluster := env("TRS_CLUSTER_ID", "local")
		if err = svc.Register(ctx, core.Identity{Subject: "controller-bootstrap", Role: "operator"}, core.Cluster{ID: cluster, Environment: "test"}); err != nil {
			return err
		}
		if err = controllers.Setup(mgr, svc, db, cluster); err != nil {
			return err
		}
		go func() { failures <- mgr.Start(ctx) }()
	}
	srv := &http.Server{Addr: env("TRS_LISTEN", "127.0.0.1:8080"), Handler: api.Handler(), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 6 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16384}
	go func() {
		slog.Info("api listening", "address", srv.Addr, "auth", "local development tokens")
		failures <- srv.ListenAndServe()
	}()
	select {
	case <-ctx.Done():
	case err = <-failures:
		cancel()
	}
	stop, done := context.WithTimeout(context.Background(), 10*time.Second)
	defer done()
	shutdownErr := srv.Shutdown(stop)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return shutdownErr
}
func main() {
	if err := run(); err != nil {
		slog.Error("shutdown", "error", err)
		os.Exit(1)
	}
}
