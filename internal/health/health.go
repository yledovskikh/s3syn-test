package health

import (
	"net/http"
	"s3syn-test/internal/config"
	"s3syn-test/internal/s3lib"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"log/slog"
)

// HealthChecker содержит конфигурацию для проверки здоровья приложения.
type HealthChecker struct {
	Logger *slog.Logger
	Sess   *session.Session
	Bucket string
}

// NewHealthChecker создает новый экземпляр HealthChecker.
func NewHealthChecker(cfg *config.Config) (*HealthChecker, error) {
	sess, err := s3lib.CreateSessionWithHTTP2(cfg)
	if err != nil {
		return nil, err
	}
	return &HealthChecker{
		Logger: cfg.Logger,
		Sess:   sess,
		Bucket: cfg.S3Bucket,
	}, nil
}

// HandleLiveness обрабатывает liveness probe.
func (h *HealthChecker) HandleLiveness(w http.ResponseWriter, r *http.Request) {
	h.Logger.Info("Liveness probe succeeded")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// HandleReadiness обрабатывает readiness probe.
func (h *HealthChecker) HandleReadiness(w http.ResponseWriter, r *http.Request) {
	if h.Sess == nil {
		h.Logger.Error("AWS session is not initialized")
		http.Error(w, "AWS session is not initialized", http.StatusServiceUnavailable)
		return
	}

	svc := s3.New(h.Sess)
	_, err := svc.ListBuckets(nil)
	if err != nil {
		h.Logger.Error("Failed to connect to S3", slog.Any("error", err))
		http.Error(w, "Failed to connect to S3", http.StatusServiceUnavailable)
		return
	}

	h.Logger.Info("Readiness probe succeeded")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
