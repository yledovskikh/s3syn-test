package main

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log/slog"
	"net/http"
	"os"
	"s3syn-test/internal/health"
	"s3syn-test/internal/metrics"
	"sync"
	"time"

	"s3syn-test/internal/config"
	"s3syn-test/internal/s3lib"
)

func main() {
	cfg := config.MustLoad()
	metrics.Init()
	// Инициализация HTTP сервера для метрик и health checks
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	// Регистрируем health handlers
	healthChecker, err := health.NewHealthChecker(cfg)
	if err != nil {
		cfg.Logger.Error("Failed to init health checker", slog.Any("error", err))
		os.Exit(1)
	}
	mux.HandleFunc("/healthz", healthChecker.HandleLiveness)
	mux.HandleFunc("/ready", healthChecker.HandleReadiness)

	go func() {
		cfg.Logger.Info("Starting metrics and health server on :8080")
		if err = http.ListenAndServe(":8080", mux); err != nil {
			cfg.Logger.Error("Failed to start metrics and health server", slog.Any("error", err))
			os.Exit(2)
		}
	}()
	for {
		var wg sync.WaitGroup
		for i, fileName := range cfg.FileNames {
			wg.Add(1)
			go func(i int, fileName string) {
				defer wg.Done()
				s3lib.ProcessFile(cfg, cfg.TempFiles[i], fileName, cfg.FileSizesBytes[i], cfg.UploadTimeoutSecs[i], cfg.DownloadTimeoutSecs[i], cfg.DeleteTimeoutSecs[i])
			}(i, fileName)
		}
		wg.Wait()
		time.Sleep(5 * time.Second)
	}
}
