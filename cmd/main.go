package main

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log/slog"
	"net/http"
	"os"
	"s3syn-test/internal/metrics"
	"sync"
	"time"

	"s3syn-test/internal/config"
	"s3syn-test/internal/s3"
)

func main() {
	cfg := config.MustLoad()
	metrics.Init()
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		cfg.Logger.Info("Starting metrics server on :8080")
		if err := http.ListenAndServe(":8080", nil); err != nil {
			cfg.Logger.Error("Failed to start metrics server", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	for {
		var wg sync.WaitGroup
		for i, fileName := range cfg.FileNames {
			wg.Add(1)
			go func(i int, fileName string) {
				defer wg.Done()
				s3.ProcessFile(cfg, cfg.TempFiles[i], fileName, cfg.FileSizesBytes[i], cfg.UploadTimeoutSecs[i], cfg.DownloadTimeoutSecs[i], cfg.DeleteTimeoutSecs[i])
			}(i, fileName)
		}
		wg.Wait()
		time.Sleep(5 * time.Second)
	}
}
