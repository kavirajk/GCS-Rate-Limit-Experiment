package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/grafana/dskit/backoff"
	loki_aws "github.com/grafana/loki/pkg/storage/chunk/aws"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")
var bucketName = flag.String("bucket", "", "s3 bucket to read/write to.")
var period = flag.Duration("period", 5*time.Minute, "Time period in minutes used for chunk object sharding.")
var region = flag.String("region", "", "s3 region.")
var withJitter = flag.Bool("with-jitter", false, "Toggle to include jitter for period sharding")
var shardFactor = flag.Int("shard-factor", 1, "shard factor to use")
var accessKey = os.Getenv("S3_ACCESS_KEY")
var secretKey = os.Getenv("S3_SECRET_KEY")

var metric = promauto.With(prometheus.DefaultRegisterer).NewCounterVec(prometheus.CounterOpts{
	Namespace: "loki",
	Name:      "s3_requests_total",
}, []string{"err", "bucket", "shard"})

func createS3ObjectClient(bucketName string, region string, accessKey string, secret string) (*loki_aws.S3ObjectClient, error) {
	conf := loki_aws.S3Config{
		BucketNames:     bucketName,
		Region:          region,
		AccessKeyID:     accessKey,
		SecretAccessKey: secret,
		Insecure:        true,
	}

	client, err := loki_aws.NewS3ObjectClient(conf)
	if err != nil {
		return nil, fmt.Errorf("error: %v", err)
	}
	return client, nil
}

type key struct {
	bucket, shard, fprint uint64
}

func (k key) String() string {
	// <user>/<period>/<shard>/fprint
	return fmt.Sprintf("user/%d/%d/%x", k.bucket, k.shard, k.fprint)

}

func newKey() key {
	fprint := rand.Uint64()
	from := time.Now().UnixNano()
	shard := fprint % uint64(*shardFactor)
	bucket := uint64(from) / uint64(*period)

	if *withJitter {
		jitter := fprint % uint64(*period)
		bucket = (uint64(from) + jitter) / uint64(*period)
	}

	return key{
		bucket: bucket,
		shard:  shard,
		fprint: fprint,
	}

}

func putObject(client *loki_aws.S3ObjectClient, key key) error {
	err := client.PutObject(context.Background(), key.String(), bytes.NewReader([]byte("hi")))
	metric.WithLabelValues(fmt.Sprint(err == nil), fmt.Sprint(key.bucket), fmt.Sprint(key.shard)).Inc()
	return err
}

func main() {
	flag.Parse()

	client, err := createS3ObjectClient(*bucketName, *region, accessKey, secretKey)
	if err != nil {
		fmt.Printf("%+v", err)
		os.Exit(1)
	}

	term := make(chan os.Signal)
	signal.Notify(term, syscall.SIGINT, syscall.SIGTERM)

	for i := 0; i < 100; i++ {
		go func() {
			// Retry with exponential backoff per AWS documentation
			backoffConfig := backoff.Config{
				MinBackoff: 100 * time.Millisecond,
				MaxBackoff: 5 * time.Second,
				MaxRetries: 10,
			}

			for {
				select {
				case <-term:
					return
				default:
					retries := backoff.New(context.Background(), backoffConfig)
					key := newKey()
					for retries.Ongoing() {
						err := putObject(client, key)
						if err != nil {
							retries.Wait()
							continue
						}
						break
					}
				}
			}
		}()
	}

	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal(http.ListenAndServe(*addr, nil))
	}()
	<-term
}
