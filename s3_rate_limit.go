package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/grafana/dskit/backoff"
	loki_aws "github.com/grafana/loki/pkg/storage/chunk/aws"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/model"
)

var addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")
var bucketName = flag.String("bucket", "", "s3 bucket to read/write to.")
var period = flag.Int64("period", 60, "Time period in minutes used for chunk object sharding.")
var region = flag.String("region", "", "s3 region.")
var accessKey = os.Getenv("S3_ACCESS_KEY")
var secretKey = os.Getenv("S3_SECRET_KEY")

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

func putObjectBatch(client *loki_aws.S3ObjectClient, keys []string) error {
	for _, key := range keys {
		if err := client.PutObject(context.Background(), key, bytes.NewReader([]byte("hi"))); err != nil {
			return err
		}
	}
	return nil
}

// func deleteObjectBatch(client *loki_aws.S3ObjectClient, keys []string) error {
// 	for _, key := range keys {
// 		if err := client.DeleteObject(context.Background(), key); err != nil {
// 			return err
// 		}
// 	}
// 	return nil
// }

func chunkKey(withJitter bool, period time.Duration) (string, error) {
	from := uint64(time.Now().UTC().UnixMilli())
	uuid := uuid.New()

	fingerPrint, err := model.FingerprintFromString(uuid.String())
	if err != nil {
		return "", err
	}
	shard := uint64(fingerPrint) % 2

	if withJitter {
		jitter := uint64(fingerPrint) % uint64(period)
		prefix := (from + jitter) % uint64(period)
		return fmt.Sprintf("baz/%x/%x/%x/%x:%s", prefix, shard, fingerPrint, from, uuid), nil
	} else {
		return fmt.Sprintf("baz/%x/%x/%x/%x:%s", uint64(period), shard, fingerPrint, from, uuid), nil
	}
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
					// Without jitter for now
					key, err := chunkKey(false, time.Duration((*period))*time.Minute)
					if err != nil {
						fmt.Printf("%+v", err)
						os.Exit(1)
					}

					retries := backoff.New(context.Background(), backoffConfig)
					for retries.Ongoing() {
						err := putObjectBatch(client, []string{key})
						if err != nil {
							fmt.Printf("%+v", err)
							retries.Wait()
						} else if err == nil {
							break
						}
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
