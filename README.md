# S3 File Operations Tool

This tool is designed to perform operations (upload, download, delete) on files stored in an S3-compatible storage. It also includes integrity checks and Prometheus metrics for monitoring the performance and success of these operations.

## Features

- **Upload files to S3**: Supports both single-part and multipart uploads.
- **Download files from S3**: Downloads files and checks their integrity.
- **Delete files from S3**: Removes files from the S3 bucket.
- **Integrity checks**: Verifies the integrity of downloaded files using MD5 checksums.
- **Prometheus metrics**: Exposes metrics for upload, download, and delete operations, including duration and success/failure status.
- **Graceful shutdown**: Handles cleanup of temporary files on program termination.

## Prerequisites

- Go 1.20 or higher
- AWS SDK for Go
- Prometheus client library for Go
- S3-compatible storage (e.g., AWS S3, MinIO)

## Configuration

The tool is configured using environment variables:

- `S3_ENDPOINT`: The endpoint URL for the S3-compatible storage.
- `S3_REGION`: The region of the S3 bucket.
- `S3_ACCESS_KEY`: The access key for the S3 bucket.
- `S3_SECRET_KEY`: The secret key for the S3 bucket.
- `S3_BUCKET`: The name of the S3 bucket.
- `FILE_PATTERNS`: Comma-separated list of file names to operate on.
- `FILE_SIZES`: Comma-separated list of file sizes in kilobytes.
- `UPLOAD_TIMEOUTS`: Comma-separated list of upload timeouts in seconds.
- `DOWNLOAD_TIMEOUTS`: Comma-separated list of download timeouts in seconds.
- `DELETE_TIMEOUTS`: Comma-separated list of delete timeouts in seconds.
- `FILES_DIR`: Directory to store temporary files (defaults to current directory).
- `LOG_FORMAT`: Log format (`text` or `json`).
- `LOG_LEVEL`: Log level (`debug`, `info`, `warn`, `error`).

## Usage

1. **Set up environment variables**:
   ```sh
   export S3_ENDPOINT="https://s3.example.com"
   export S3_REGION="us-east-1"
   export S3_ACCESS_KEY="your-access-key"
   export S3_SECRET_KEY="your-secret-key"
   export S3_BUCKET="your-bucket-name"
   export FILE_PATTERNS="file1.txt,file2.txt"
   export FILE_SIZES="1024,2048"
   export UPLOAD_TIMEOUTS="5,10"
   export DOWNLOAD_TIMEOUTS="5,10"
   export DELETE_TIMEOUTS="5,10"
   export FILES_DIR="/tmp/files"
   export LOG_FORMAT="json"
   export LOG_LEVEL="info"
   ```
2. **Run the program:**
```
go run main.go
```

3. **Access Prometheus metrics:**

- The metrics server runs on http://localhost:8080/metrics

## Usage

The following Prometheus metrics are exposed:

 - s3_upload_duration_seconds: Time taken to upload a file to S3.
 - s3_download_duration_seconds: Time taken to download a file from S3.
 - s3_delete_duration_seconds: Time taken to delete a file from S3.
 - s3_file_is_incorrect: File integrity check (0 if OK, 1 if corrupted).
 - s3_operation_timeout: Operation timeout exceeded (1 if timeout exceeded, 0 otherwise).

## Example

To upload, download, and delete two files (file1.txt and file2.txt) with sizes 1MB and 2MB respectively, with timeouts of 5 and 10 seconds for each operation:

```
export FILE_PATTERNS="file1.txt,file2.txt"
export FILE_SIZES="1024,2048"
export UPLOAD_TIMEOUTS="5,10"
export DOWNLOAD_TIMEOUTS="5,10"
export DELETE_TIMEOUTS="5,10"
go run main.go
```
### Graceful Shutdown

The program handles SIGINT and SIGTERM signals to perform a graceful shutdown, cleaning up temporary files before exiting.

### License

This project is licensed under the MIT License.