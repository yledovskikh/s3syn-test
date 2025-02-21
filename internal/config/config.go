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
	S3Endpoint              string `env:"S3_ENDPOINT" env-default:"info"`
	S3Region                string `env:"S3_REGION"`
	S3AccessKey             string `env:"S3_ACCESS_KEY"`
	S3SecretKey             string `env:"S3_SECRET_KEY"`
	S3Bucket                string `env:"S3_BUCKET"`
	FilePatterns            string `env:"FILE_PATTERNS"` // Формат: "file1.txt,file2.txt"
	FileSizes               string `env:"FILE_SIZES"`    // Формат: "1024,2048" (в байтах)
	UploadTimeouts          string `env:"UPLOAD_TIMEOUTS"`
	DownloadTimeouts        string `env:"DOWNLOAD_TIMEOUTS"`
	DeleteTimeouts          string `env:"DELETE_TIMEOUTS"`
	FilesDir                string `env:"FILES_DIR" env-default:"/tmp"`
	LogFormat               string `env:"LOG_FORMAT" env-default:"json"`
	LogLevel                string `env:"LOG_LEVEL" env-default:"info"`
	MinFileSizeForMultipart int    `env:"MIN_FILE_SIZE_FOR_MULTIPART" env-default:"8388608"` // 8 MB
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
	if env.LogLevel == "debug" {
		cfg.AwsLogLevel = aws.LogDebug
	} else {
		cfg.AwsLogLevel = aws.LogOff
	}

	cfg.Logger = initLogger(env.LogLevel, env.LogFormat)
	cfg.FileNames = cfg.parseCSV(env.FilePatterns)
	cfg.FileSizesBytes = cfg.parseIntCSV(env.FileSizes)
	cfg.UploadTimeoutSecs = cfg.parseIntCSV(env.UploadTimeouts)
	cfg.DownloadTimeoutSecs = cfg.parseIntCSV(env.DownloadTimeouts)
	cfg.DeleteTimeoutSecs = cfg.parseIntCSV(env.DeleteTimeouts)

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
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
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
	tempFile, err := os.Create(filepath.Join(cfg.FilesDir, fileName))
	if err != nil {
		cfg.Logger.Error("Failed to create temporary file", slog.Any("error", err))
		os.Exit(1)
	}
	defer func(tempFile *os.File) {
		err = tempFile.Close()
		if err != nil {
		}
	}(tempFile)

	data := make([]byte, size) // Размер в байтах
	n, err := tempFile.Write(data)
	if err != nil || n != len(data) {
		cfg.Logger.Error("Failed to write to temporary file", slog.Any("error", err))
		os.Exit(1)
	}
	return tempFile.Name()
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
