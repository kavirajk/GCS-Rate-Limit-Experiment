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
	loki_gcs "github.com/grafana/loki/pkg/storage/chunk/client/gcp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// SET credentials via `GOOGLE_APPLICATION_CREDENTIALS`

var addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests")
var bucketName = flag.String("bucket", "", "gcs bucket to read/write to")
var maxBackoff = flag.Duration("max-backoff", 10*time.Second, "Max backoff period")
var period = flag.Duration("period", 5*time.Minute, "Time period in minutes used for chunk object sharding")
var shardFactor = flag.Int("shard-factor", 1, "Shard factor to use")
var withJitter = flag.Bool("with-jitter", false, "Toggle to include jitter for period sharding")

var metric = promauto.With(prometheus.DefaultRegisterer).NewCounterVec(prometheus.CounterOpts{
	Namespace: "loki",
	Name:      "gcs_requests_total",
}, []string{"err", "bucket", "shard"})

// TODO(kavi): Modify Key to compley with GCS.
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

func createGCSObjectClient(bucketName string) (*loki_gcs.GCSObjectClient, error) {
	conf := loki_gcs.GCSConfig{
		BucketName: bucketName,
		Insecure:   true,
	}

	client, err := loki_gcs.NewGCSObjectClient(conf)
	if err != nil {
		return nil, fmt.Errorf("error: %v", err)
	}
	return client, nil
}

func putObject(client *loki_gcs.GCSObjectClient, key key) error {
	err := client.PutObject(context.Background(), key.String(), bytes.NewReader([]byte("hi")))
	// TODO: Metric similar to our prod gcs_storage_*. To get visibility on actual status code to filter out rate_limited vs other errors?
	metric.WithLabelValues(fmt.Sprint(err != nil), fmt.Sprint(key.bucket), fmt.Sprint(key.shard)).Inc()
	return err
}

func main() {
	flag.Parse()

	client, err := createGCSObjectClient(*bucketName)
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
				MaxBackoff: *maxBackoff,
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
