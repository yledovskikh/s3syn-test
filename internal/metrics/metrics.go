package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	UploadDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "s3_upload_duration_seconds",
		Help: "Time taken to upload file to S3",
	}, []string{"file"})
	DownloadDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "s3_download_duration_seconds",
		Help: "Time taken to download file from S3",
	}, []string{"file"})
	DeleteDuration = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "s3_delete_duration_seconds",
		Help: "Time taken to delete file from S3",
	}, []string{"file"})
	FileIsCorrected = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "s3_file_is_incorrect",
		Help: "File integrity check (1 if OK, 0 if corrupted)",
	}, []string{"file"})
	TimeoutMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "s3_operation_timeout",
		Help: "Operation timeout exceeded (1 if timeout exceeded, 0 otherwise)",
	}, []string{"file", "operation"})
	IsError = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "s3_operation_is_error",
		Help: "An error occurred while performing the operation (1 if error occurred, 0 otherwise)",
	}, []string{"file", "operation"})
)

func Init() {
	prometheus.MustRegister(UploadDuration)
	prometheus.MustRegister(DownloadDuration)
	prometheus.MustRegister(DeleteDuration)
	prometheus.MustRegister(FileIsCorrected)
	prometheus.MustRegister(TimeoutMetric)
	prometheus.MustRegister(IsError)
}
