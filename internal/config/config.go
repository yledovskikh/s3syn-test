package config

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/ilyakaznacheev/cleanenv"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

type EnvData struct {
	S3Endpoint              string `env:"S3_ENDPOINT"`
	S3Region                string `env:"S3_REGION"`
	S3AccessKey             string `env:"S3_ACCESS_KEY"`
	S3SecretKey             string `env:"S3_SECRET_KEY"`
	S3Bucket                string `env:"S3_BUCKET"`
	FilePatterns            string `env:"FILE_PATTERNS" env-default:"file1kb,file1mb"` // Формат: "file1.txt,file2.txt"
	FileSizes               string `env:"FILE_SIZES" env-default:"1024,1048576"`       // Формат: "1024,2048" (в байтах)
	UploadTimeouts          string `env:"UPLOAD_TIMEOUTS" env-default:"1,1"`
	DownloadTimeouts        string `env:"DOWNLOAD_TIMEOUTS" env-default:"1,1"`
	DeleteTimeouts          string `env:"DELETE_TIMEOUTS" env-default:"1,1"`
	FilesDir                string `env:"FILES_DIR" env-default:"/tmp"`
	LogFormat               string `env:"LOG_FORMAT" env-default:"json"`
	LogLevel                string `env:"LOG_LEVEL" env-default:"info"`
	MinFileSizeForMultipart int    `env:"MIN_FILE_SIZE_FOR_MULTIPART" env-default:"8388608"` // 8 MB
	TaskInterval            int    `env:"TASK_INTERVAL" env-default:"60"`
	Profiler                bool   `env:"PROFILER" env-default:"false"`
	ConcurrencyMPU          int    `env:"CONCURRENCY_MPU" env-default:"3"`
}

type Config struct {
	AwsLogLevel             aws.LogLevelType
	TempFiles               []string
	Logger                  *slog.Logger
	FilesDir                string
	FileNames               []string
	FileSizesBytes          []int
	UploadTimeoutSecs       []int
	DownloadTimeoutSecs     []int
	DeleteTimeoutSecs       []int
	S3Endpoint              string
	S3Region                string
	S3AccessKey             string
	S3SecretKey             string
	S3Bucket                string
	MinFileSizeForMultipart int
	TaskInterval            int
	ConcurrencyMPU          int
	Profiler                bool
}

func MustLoad() *Config {
	var env EnvData
	err := cleanenv.ReadEnv(&env)
	if err != nil {
		log.Fatal("cannot parse config from env", err)
	}
	var cfg Config
	cfg.FilesDir = env.FilesDir
	cfg.S3Endpoint = env.S3Endpoint
	cfg.S3Bucket = env.S3Bucket
	cfg.S3Region = env.S3Region
	cfg.S3AccessKey = env.S3AccessKey
	cfg.S3SecretKey = env.S3SecretKey
	cfg.MinFileSizeForMultipart = env.MinFileSizeForMultipart
	cfg.TaskInterval = env.TaskInterval
	cfg.Profiler = env.Profiler
	cfg.ConcurrencyMPU = env.ConcurrencyMPU
	if env.LogLevel == "debug" {
		cfg.AwsLogLevel = aws.LogDebug
	} else {
		cfg.AwsLogLevel = aws.LogOff
	}

	cfg.Logger = initLogger(env.LogLevel, env.LogFormat)
	cfg.Logger.Debug("log format - " + env.LogFormat)
	cfg.Logger.Debug("FilePatterns - " + env.FilePatterns)
	cfg.FileNames = cfg.parseCSV(env.FilePatterns)
	cfg.Logger.Debug("FileSizes - " + env.FileSizes)
	cfg.FileSizesBytes = cfg.parseIntCSV(env.FileSizes)
	cfg.Logger.Debug("UploadTimeouts - " + env.UploadTimeouts)
	cfg.UploadTimeoutSecs = cfg.parseIntCSV(env.UploadTimeouts)
	cfg.Logger.Debug("DownloadTimeouts - " + env.DownloadTimeouts)
	cfg.DownloadTimeoutSecs = cfg.parseIntCSV(env.DownloadTimeouts)
	cfg.Logger.Debug("DeleteTimeouts - " + env.DeleteTimeouts)
	cfg.DeleteTimeoutSecs = cfg.parseIntCSV(env.DeleteTimeouts)
	cfg.Logger.Debug("s3 endpoint - " + env.S3Endpoint)
	cfg.Logger.Debug("FILES_DIR - " + env.FilesDir)
	cfg.Logger.Debug("s3 MinFileSizeForMultipart - " + strconv.Itoa(env.MinFileSizeForMultipart))
	if len(cfg.FileNames) != len(cfg.FileSizesBytes) || len(cfg.FileNames) != len(cfg.UploadTimeoutSecs) || len(cfg.FileNames) != len(cfg.DownloadTimeoutSecs) || len(cfg.FileNames) != len(cfg.DeleteTimeoutSecs) {
		cfg.Logger.Error("Mismatch in the number of files, sizes, or timeouts specified")
		os.Exit(1)
	}
	cfg.checkAndRemoveExistingFiles()
	cfg.createTempFiles()
	cfg.setupGracefulShutdown()
	return &cfg
}

func initLogger(logLevel, logFormat string) *slog.Logger {
	var level slog.Level
	switch logLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler
	if logFormat == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level, AddSource: true})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level, AddSource: true})
	}
	return slog.New(handler)
}

func (cfg *Config) parseCSV(input string) []string {
	return strings.Split(strings.TrimSpace(input), ",")
}

func (cfg *Config) parseIntCSV(input string) []int {
	parts := cfg.parseCSV(input)
	ints := make([]int, len(parts))
	for i, part := range parts {
		num, err := strconv.Atoi(part)
		if err != nil {
			cfg.Logger.Error("Invalid integer value in CSV", slog.String("value", part), slog.Any("error", err))
			os.Exit(1)
		}
		ints[i] = num
	}
	return ints
}

func (cfg *Config) checkAndRemoveExistingFiles() {
	for _, fileName := range cfg.FileNames {
		matchedFiles, err := filepath.Glob(filepath.Join(cfg.FilesDir, fmt.Sprintf("%s-*", fileName)))
		if err != nil {
			cfg.Logger.Warn("Error while checking for existing files", slog.String("file", fileName), slog.Any("error", err))
			continue
		}
		for _, filePath := range matchedFiles {
			err = os.Remove(filePath)
			if err != nil {
				cfg.Logger.Warn("Failed to remove existing file", slog.String("file", filePath), slog.Any("error", err))
			} else {
				cfg.Logger.Info("Removed existing file", slog.String("file", filePath))
			}
		}
	}
}

func (cfg *Config) createTempFiles() {
	for i, fileName := range cfg.FileNames {
		tempFilePath := cfg.createTempFileWithSize(fileName, cfg.FileSizesBytes[i])
		cfg.TempFiles = append(cfg.TempFiles, tempFilePath)
		cfg.Logger.Info("Created temporary file", slog.String("file", tempFilePath), slog.Int("size", cfg.FileSizesBytes[i]))
	}
}

func (cfg *Config) createTempFileWithSize(fileName string, size int) string {
	// Формируем путь к временному файлу
	tempFilePath := filepath.Join(cfg.FilesDir, fileName)

	// Создаем временный файл
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		cfg.Logger.Error("Failed to create temporary file", slog.Any("error", err))
		os.Exit(1)
	}
	defer func(tempFile *os.File) {
		err := tempFile.Close()
		if err != nil {
			cfg.Logger.Warn("Failed to close temporary file", slog.String("file", tempFilePath), slog.Any("error", err))
		}
	}(tempFile)

	// Размер блока записи (например, 4 KB)
	blockSize := 4 * 1024 // 4 KB
	written := 0

	// Создаем буфер фиксированного размера
	data := make([]byte, blockSize)

	// Записываем данные блоками
	for written < size {
		bytesToWrite := blockSize
		if remaining := size - written; remaining < blockSize {
			bytesToWrite = remaining
		}

		n, err := tempFile.Write(data[:bytesToWrite])
		if err != nil || n != bytesToWrite {
			cfg.Logger.Error("Failed to write to temporary file", slog.Any("error", err))
			os.Exit(1)
		}

		written += n
	}

	// Возвращаем путь к созданному файлу
	return tempFilePath
}

func (cfg *Config) setupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cfg.Logger.Info("Gracefully shutting down...")
		for _, tempFile := range cfg.TempFiles {
			err := os.Remove(tempFile)
			if err != nil {
				cfg.Logger.Warn("Failed to remove temporary file", slog.String("file", tempFile), slog.Any("error", err))
			} else {
				cfg.Logger.Info("Removed temporary file", slog.String("file", tempFile))
			}
		}
		os.Exit(0)
	}()
}
