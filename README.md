# S3 SYNTHETIC TEST

Данное приложение предназначено для тестирования функциональности S3-совместимого хранилища. 
Приложение периодически выполняет - загрузку, скачивание, удаление и проверку целостности файлов.

Время каждой операции фиксируется и может быть получено в формате Prometheus metrics

Данный сервис поддерживает возможность запуска тестов сразу несколько файлов. Для этого нужно указать через запятую значения для соответствующих переменных:
FILE_PATTERNS, FILE_SIZES, UPLOAD_TIMEOUTS, DOWNLOAD_TIMEOUTS, DELETE_TIMEOUTS.

Это может быть полезно если нужно протестировать работу S3 в разных режимах работы с разным размером объектов.

## Требования

 - Go 1.24+
   - AWS SDK for Go v1  
   - Библиотека Prometheus client_golang для Go
   - Библиотека cleanenv для парсинга переменных окружения
 - S3 совместимое с хранилище 

## Конфигурация
Приложение использует переменные окружения для конфигурации. Ниже приведены доступные параметры:

### Конфигурация приложения
#### Обязательные параметры:

| Переменная окружения          | Описание                                                       | Значение по умолчанию |
|-------------------------------|----------------------------------------------------------------|-----------------------|
| `S3_ENDPOINT`                 | URL эндпоинта S3                                               | `info`                |
| `S3_REGION`                   | Регион S3                                                      |                       |
| `S3_ACCESS_KEY`               | Ключ доступа S3                                                |                       |
| `S3_SECRET_KEY`               | Секретный ключ S3                                              |                       |
| `S3_BUCKET`                   | Имя бакета S3                                                  |                       |

#### Опциональные параметры:
| Переменная окружения          | Описание                                                       | Значение по умолчанию |
|-------------------------------|----------------------------------------------------------------|-----------------------|
| `FILE_PATTERNS`               | Список имен файлов через запятую                               | `file1kb,file1mb`     |
| `FILE_SIZES`                  | Размеры файлов в байтах через запятую                          | `1024,1048576`        |
| `UPLOAD_TIMEOUTS`             | Таймауты загрузки в секундах через запятую                     | `1,1`                 |
| `DOWNLOAD_TIMEOUTS`           | Таймауты скачивания в секундах через запятую                   | `1,1`                 |
| `DELETE_TIMEOUTS`             | Таймауты удаления в секундах через запятую                     | `1,1`                 |
| `FILES_DIR`                   | Директория для временных файлов                                | `/tmp`                |
| `LOG_FORMAT`                  | Формат логов (`json` или `text`)                               | `json`                |
| `LOG_LEVEL`                   | Уровень логирования (`debug`, `info`, `warn`, `error`)         | `info`                |
| `MIN_FILE_SIZE_FOR_MULTIPART` | Минимальный размер файла для многопоточной загрузки (в байтах) | `8388608` (8 MB)      |
| `TASK_INTERVAL`               | Интервал между выполнением задач в секундах                    | `60`                  |
| `PROFILER`                    | Включение профилировщика                                       | `false`               |
| `CONCURRENCY_MPU`             | Количество параллельных потоков при загрузке multipart upload  | `3`                   |

### Важно:
 - Количество элементов в FILE_PATTERNS, FILE_SIZES, UPLOAD_TIMEOUTS, DOWNLOAD_TIMEOUTS и DELETE_TIMEOUTS должно быть одинаковым.
 - Для больших файлов (> 8 MB) автоматически используется multipart upload.

### Запуск приложения
#### Предварительные требования
Убедитесь, что необходимые переменные окружения установлены перед запуском приложения.
### Переменные окружения
```bash
export S3_ENDPOINT=http://localhost:9000
export S3_ACCESS_KEY=minioadmin
export S3_SECRET_KEY=minioadmin
export S3_BUCKET=mybucket
export FILE_PATTERNS=file1kb,file1mb
export FILE_SIZES=1024,1048576
export UPLOAD_TIMEOUTS=5,10
export DOWNLOAD_TIMEOUTS=5,10
export DELETE_TIMEOUTS=5,10
export FILES_DIR=/tmp
export LOG_FORMAT=json
export LOG_LEVEL=info
export MIN_FILE_SIZE_FOR_MULTIPART=8388608
export TASK_INTERVAL=60
export PROFILER=false
```
## Сборка и запуск
```
go build -o s3syn-test main.go
./s3syn-test
```
## С помощью Podman
```
podman build -t s3syn-test .
podman run -e S3_ENDPOINT=... -e S3_ACCESS_KEY=... -e S3_SECRET_KEY=... -p 8080:8080 s3syn-test
```
## Метрики

Приложение предоставляет метрики на порту 8080 по пути /metrics. Доступны следующие метрики:

- s3_upload_duration_seconds: Время выполнения операции загрузки файла в S3.
- s3_download_duration_seconds: Время выполнения операции скачивания файла из S3.
- s3_delete_duration_seconds: Время выполнения операции удаления файла из S3.
- s3_file_is_correct: Результат проверки целостности файла (1 если корректен, 0 если поврежден).
- s3_operation_timeout: Указывает, произошел ли таймаут операции (1 если да, 0 если нет).
- s3_operation_is_error: Указывает, произошла ли ошибка во время операции (1 если да, 0 если нет).
## Проверка работоспособности
Приложение предоставляет два endpoint для проверки состояния:
- /healthz: Проверка работоспособности (liveness probe).
- /ready: Проверка готовности принимать нагрузку (readiness probe).

Оба endpoint возвращают 200 OK, если все хорошо.

## Профилирование
Для включения профилировщика установите переменную окружения PROFILER в значение true. Профилировщик будет доступен на порту 6060.
Пример подключения к endpoint профайлера
```
go tool pprof -http=:8081 http://localhost:6060/debug/pprof/heap
```
## Логирование
Приложение поддерживает как формат json, так и текстовый формат логов. Уровень логирования можно настроить с помощью переменной окружения LOG_LEVEL.
