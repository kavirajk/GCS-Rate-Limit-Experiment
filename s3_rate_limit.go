package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
	loki_aws "github.com/grafana/loki/pkg/storage/chunk/aws"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var addr = flag.String("listen-address", ":8080", "The address to listen on for HTTP requests.")
var bucketName = flag.String("bucket", "", "s3 bucket to read/write to.")
var region = flag.String("region", "", "s3 region.")
var accessKey = os.Getenv("S3_ACCESS_KEY")
var secretKey = os.Getenv("S3_SECRET_KEY")
var timeToRun = flag.Uint64("time-to-run", 5, "The amount of time to run the experiment in seconds.")
var tickInterval = flag.Uint64("tick-interval", 200, "Interval between requests in milliseconds.")

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

func deleteObjectBatch(client *loki_aws.S3ObjectClient, keys []string) error {
	for _, key := range keys {
		if err := client.DeleteObject(context.Background(), key); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	flag.Parse()

	var keys []string
	ticker := time.NewTicker(time.Duration(*tickInterval) * time.Millisecond)
	done := make(chan bool)

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				client, err := createS3ObjectClient(*bucketName, *region, accessKey, secretKey)
				if err != nil {
					fmt.Printf("%+v", err)
					os.Exit(1)
				}

				id := uuid.New()
				key := fmt.Sprintf("foo/%s/%s", id, id)
				keys = append(keys, key)

				if err := putObjectBatch(client, []string{key}); err != nil {
					fmt.Printf("%+v", err)
					os.Exit(1)
				}
			}
		}
	}()

	go func() {
		// Run the experiment for timeToRun
		time.Sleep(time.Duration(*timeToRun) * time.Second)
		ticker.Stop()
		done <- true

		// Clean up the objects we wrote when we are done
		client, err := createS3ObjectClient(*bucketName, *region, accessKey, secretKey)
		if err != nil {
			fmt.Printf("%+v", err)
			os.Exit(1)
		}
		if err := deleteObjectBatch(client, keys); err != nil {
			fmt.Printf("%+v", err)
			os.Exit(1)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*addr, nil))
}
