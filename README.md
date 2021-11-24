# S3 Rate Limit Experiment

Wrap [Loki's `S3ObjectClient`](https://github.com/grafana/loki/blob/main/pkg/storage/chunk/aws/s3_storage_client.go) and generate write load via `PutObject()` spam across 100 Goroutines per replica.

Retry with configurable exponential backoff and jitter from [dskit](https://github.com/grafana/dskit/tree/main/backoff).

## Usage

```shell
$ ./s3-rate-limit-experiment --help
Usage of ./s3-rate-limit-experiment:
  -bucket string
        s3 bucket to read/write to.
  -listen-address string
        The address to listen on for HTTP requests. (default ":8080")
  -max-backoff duration
        Max backoff period. (default 10s)
  -period duration
        Time period in minutes used for chunk object sharding. (default 5m0s)
  -region string
        s3 region.
  -shard-factor int
        shard factor to use (default 1)
  -with-jitter
        Toggle to include jitter for period sharding
```

### S3 credentials are set via the following environment variables:

```shell
S3_ACCESS_KEY
S3_SECRET_KEY
```

## Observe Related Metrics

```shell
$ curl -sq localhost:8080/metrics | grep s3            
# HELP loki_s3_request_duration_seconds Time spent doing S3 requests.
# TYPE loki_s3_request_duration_seconds histogram
loki_s3_request_duration_seconds_bucket{operation="S3.PutObject",status_code="200",le="0.025"} 0
loki_s3_request_duration_seconds_bucket{operation="S3.PutObject",status_code="200",le="0.05"} 0
loki_s3_request_duration_seconds_bucket{operation="S3.PutObject",status_code="200",le="0.1"} 0
loki_s3_request_duration_seconds_bucket{operation="S3.PutObject",status_code="200",le="0.25"} 17
loki_s3_request_duration_seconds_bucket{operation="S3.PutObject",status_code="200",le="0.5"} 17
loki_s3_request_duration_seconds_bucket{operation="S3.PutObject",status_code="200",le="1"} 17
loki_s3_request_duration_seconds_bucket{operation="S3.PutObject",status_code="200",le="2"} 18
loki_s3_request_duration_seconds_bucket{operation="S3.PutObject",status_code="200",le="+Inf"} 18
loki_s3_request_duration_seconds_sum{operation="S3.PutObject",status_code="200"} 3.2037681250000003
loki_s3_request_duration_seconds_count{operation="S3.PutObject",status_code="200"} 18
```
