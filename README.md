# S3 Rate Limit Experiment

Usage:

```shell
Usage of ./s3_rate_limit:
  -access-key string
        s3 access key
  -bucket string
        s3 bucket to read/write to.
  -listen-address string
        The address to listen on for HTTP requests. (default ":8080")
  -region string
        s3 region.
  -secret-key string
        s3 secret key
  -tick-interval uint
        Interval between requests in milliseconds. (default 200)
  -time-to-run uint
        The amount of time to run the experiment in seconds. (default 5)
```

Observe related metrics:

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
