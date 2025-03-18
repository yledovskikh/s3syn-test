package s3lib

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"log/slog"
	"s3syn-test/internal/config"
	"s3syn-test/internal/metrics"
)

func ProcessFile(cfg *config.Config, localFilePath, fileName string, fileSize, uploadTimeout, downloadTimeout, deleteTimeout int) {
	err := UploadFileToS3(cfg, localFilePath, fileName, fileSize, uploadTimeout)
	if err != nil {
		cfg.Logger.Error("Upload failed", slog.String("file", fileName), slog.Any("error", err))
		return
	}

	downloadedFilePath, err := DownloadFileFromS3(cfg, fileName, downloadTimeout)
	if err != nil {
		cfg.Logger.Error("Download failed", slog.String("file", fileName), slog.Any("error", err))
		return
	}

	CheckFileIntegrity(cfg, localFilePath, downloadedFilePath, fileName)
	err = os.Remove(downloadedFilePath)
	if err != nil {
		cfg.Logger.Warn("Failed to remove downloaded file", slog.String("file", downloadedFilePath), slog.Any("error", err))
	}

	err = DeleteFileFromS3(cfg, fileName, deleteTimeout)
	if err != nil {
		cfg.Logger.Error("Delete failed", slog.String("file", fileName), slog.Any("error", err))
		return
	}
}

func CreateSessionWithHTTP2(cfg *config.Config) (*session.Session, error) {
	tr := &http.Transport{
		TLSClientConfig:   &tls.Config{InsecureSkipVerify: true},
		ForceAttemptHTTP2: true,
	}
	client := &http.Client{Transport: tr}
	sess, err := session.NewSession(&aws.Config{
		Endpoint:         aws.String(cfg.S3Endpoint),
		Region:           aws.String(cfg.S3Region),
		Credentials:      credentials.NewStaticCredentials(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		S3ForcePathStyle: aws.Bool(true),
		HTTPClient:       client,
		LogLevel:         aws.LogLevel(cfg.AwsLogLevel),
	})
	return sess, err
}

func UploadFileToS3(cfg *config.Config, filePath, fileName string, fileSize, uploadTimeout int) error {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(uploadTimeout)*time.Second)
	defer cancel()

	sess, err := CreateSessionWithHTTP2(cfg)
	if err != nil {
		return err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var result *s3manager.UploadOutput
	if fileSize < cfg.MinFileSizeForMultipart {
		svc := s3.New(sess)
		_, err = svc.PutObjectWithContext(ctx, &s3.PutObjectInput{
			Bucket: aws.String(cfg.S3Bucket),
			Key:    aws.String(fileName),
			Body:   file,
		})
	} else {
		uploader := s3manager.NewUploader(sess, func(u *s3manager.Uploader) {
			u.PartSize = int64(cfg.MinFileSizeForMultipart) * 1024 * 1024
			u.Concurrency = cfg.ConcurrencyMPU
		})
		result, err = uploader.UploadWithContext(ctx, &s3manager.UploadInput{
			Bucket: aws.String(cfg.S3Bucket),
			Key:    aws.String(fileName),
			Body:   file,
		})
	}

	if ctx.Err() != nil {
		cfg.Logger.Warn("Upload operation timed out", slog.String("file", fileName))
		metrics.TimeoutMetric.WithLabelValues(fileName, "upload").Set(1)
		return ctx.Err()
	}

	if err != nil {
		cfg.Logger.Error("Upload failed", slog.String("file", fileName), slog.Any("error", err))
		metrics.IsError.WithLabelValues(fileName, "upload").Set(1)
		return err
	}

	duration := time.Since(start).Seconds()
	metrics.UploadDuration.WithLabelValues(fileName).Set(duration)
	metrics.IsError.WithLabelValues(fileName, "upload").Set(0)
	metrics.TimeoutMetric.WithLabelValues(fileName, "upload").Set(0)

	if result != nil {
		cfg.Logger.Info("File uploaded successfully", slog.String("file", fileName), slog.String("etag", aws.StringValue(result.ETag)))
	} else {
		cfg.Logger.Info("File uploaded successfully using PutObject", slog.String("file", fileName))
	}
	return nil
}

func DownloadFileFromS3(cfg *config.Config, fileName string, downloadTimeout int) (string, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(downloadTimeout)*time.Second)
	defer cancel()

	sess, err := CreateSessionWithHTTP2(cfg)
	if err != nil {
		return "", err
	}

	tempFileName := fmt.Sprintf("%s-tmp", fileName)
	tempFilePath := filepath.Join(cfg.FilesDir, tempFileName)
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	svc := s3.New(sess)
	resp, err := svc.GetObjectWithContext(ctx, &s3.GetObjectInput{
		Bucket: aws.String(cfg.S3Bucket),
		Key:    aws.String(fileName),
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		if ctx.Err() != nil {
			cfg.Logger.Warn("Download operation timed out", slog.String("file", fileName))
			metrics.TimeoutMetric.WithLabelValues(fileName, "download").Set(1)
			metrics.IsError.WithLabelValues(fileName, "download").Set(0)
			return "", ctx.Err()
		}
		cfg.Logger.Error("Failed to write data to temporary file", slog.String("file", fileName), slog.Any("error", err))
		metrics.IsError.WithLabelValues(fileName, "download").Set(1)
		metrics.TimeoutMetric.WithLabelValues(fileName, "download").Set(0)
		return "", err
	}
	cfg.Logger.Info("File downloaded successfully", slog.String("file", fileName))
	duration := time.Since(start).Seconds()
	metrics.DownloadDuration.WithLabelValues(fileName).Set(duration)
	metrics.TimeoutMetric.WithLabelValues(fileName, "download").Set(0)
	metrics.IsError.WithLabelValues(fileName, "download").Set(0)

	return tempFilePath, nil
}

func DeleteFileFromS3(cfg *config.Config, fileName string, deleteTimeout int) error {
	start := time.Now()
	defer func() {
		duration := time.Since(start).Seconds()
		metrics.DeleteDuration.WithLabelValues(fileName).Set(duration)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(deleteTimeout)*time.Second)
	defer cancel()

	sess, err := CreateSessionWithHTTP2(cfg)
	if err != nil {
		return err
	}

	svc := s3.New(sess)
	_, err = svc.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(cfg.S3Bucket),
		Key:    aws.String(fileName),
	})
	if ctx.Err() != nil {
		cfg.Logger.Warn("Delete operation timed out", slog.String("file", fileName))
		metrics.TimeoutMetric.WithLabelValues(fileName, "delete").Set(1)
		return ctx.Err()
	}

	if err != nil {
		cfg.Logger.Error("Delete failed", slog.String("file", fileName), slog.Any("error", err))
		metrics.IsError.WithLabelValues(fileName, "delete").Set(1)
		return err
	}

	cfg.Logger.Info("File deleted successfully", slog.String("file", fileName))
	return nil
}

func CheckFileIntegrity(cfg *config.Config, originalFilePath, downloadedFilePath, fileName string) {
	// Создаем хешеры для обоих файлов
	originalHasher := md5.New()
	downloadedHasher := md5.New()

	// Функция для вычисления хеша файла блоками
	calculateHash := func(filePath string, hasher hash.Hash) error {
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer file.Close()

		// Читаем файл блоками
		buffer := make([]byte, 4*1024) // Размер буфера: 4 KB
		for {
			n, err := file.Read(buffer)
			if err != nil && err != io.EOF {
				return err
			}
			if n == 0 {
				break
			}
			hasher.Write(buffer[:n]) // Обновляем хеш для прочитанного блока
			if err == io.EOF {
				break
			}
		}
		return nil
	}

	// Вычисляем хеши для обоих файлов
	if err := calculateHash(originalFilePath, originalHasher); err != nil {
		cfg.Logger.Error("Failed to read original file", slog.String("file", originalFilePath), slog.Any("error", err))
		return
	}

	if err := calculateHash(downloadedFilePath, downloadedHasher); err != nil {
		cfg.Logger.Error("Failed to read downloaded file", slog.String("file", downloadedFilePath), slog.Any("error", err))
		return
	}

	// Получаем результаты хеширования
	originalHash := hex.EncodeToString(originalHasher.Sum(nil))
	downloadedHash := hex.EncodeToString(downloadedHasher.Sum(nil))

	// Сравниваем хеши
	if originalHash != downloadedHash {
		cfg.Logger.Warn("File integrity check failed", slog.String("file", fileName))
		metrics.FileIsCorrected.WithLabelValues(fileName).Set(0)
	} else {
		cfg.Logger.Info("File integrity check passed", slog.String("file", fileName))
		metrics.FileIsCorrected.WithLabelValues(fileName).Set(1)
	}
}
