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

	"github.com/google/uuid"
	"github.com/grafana/dskit/backoff"
	loki_aws "github.com/grafana/loki/pkg/storage/chunk/aws"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")
var bucketName = flag.String("bucket", "", "s3 bucket to read/write to.")
var period = flag.Int64("period", 60, "Time period in minutes used for chunk object sharding.")
var region = flag.String("region", "", "s3 region.")
var withJitter = flag.Bool("with-jitter", false, "Toggle to include jitter for period sharding")
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

func chunkKey(withJitter bool, period time.Duration, shard int) (string, error) {
	from := uint64(time.Now().UTC().UnixNano())
	// simiulate a good distribution of active stream fingerprints
	chance := rand.Intn(10) + 1
	remainder := from % uint64(period)
	percent := (float64(remainder) / float64(period)) * 10
	uuid := uuid.New()

	if !withJitter {
		return fmt.Sprintf("bat/%x/%x/%s", (from - remainder), shard, uuid), nil
	} else if float64(chance) <= percent { // put this chunk in the next time period prefix
		return fmt.Sprintf("bat/%x/%x/%s", (from + (uint64(period) - remainder)), shard, uuid), nil
	} else {
		return fmt.Sprintf("bat/%x/%x/%s", (from - remainder), shard, uuid), nil
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

			prefixFactor := 1

			for {
				select {
				case <-term:
					return
				default:
					// Without jitter for now
					key, err := chunkKey(*withJitter, time.Duration((*period))*time.Minute, prefixFactor)
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

					if prefixFactor == 1 {
						prefixFactor = prefixFactor + 1
					} else if prefixFactor == 2 {
						prefixFactor = prefixFactor - 1
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
