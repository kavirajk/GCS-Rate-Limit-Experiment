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
)

var addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")
var bucketName = flag.String("bucket", "", "s3 bucket to read/write to.")
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
			prefixFactor := 1
			for {
				select {
				case <-term:
					return
				default:
					id := uuid.New()
					key := fmt.Sprintf("bar/%d/%s/%s", prefixFactor, id, id)

					// Retry with exponential backoff per AWS documentation
					backoffConfig := backoff.Config{
						MinBackoff: 100 * time.Millisecond,
						MaxBackoff: 3 * time.Second,
						MaxRetries: 10,
					}

					retries := backoff.New(context.Background(), backoffConfig)
					for retries.Ongoing() {
						if err := putObjectBatch(client, []string{key}); err != nil {
							fmt.Printf("%+v", err)
						}
						retries.Wait()
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
