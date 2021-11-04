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
var accessKey = flag.String("access-key", "", "s3 access key")
var secretKey = flag.String("secret-key", "", "s3 secret key")

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

	fmt.Printf("S3 Client Created: %+v", client)
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

	go func() {
		// bucketName, region, accessKey, secret
		client, err := createS3ObjectClient(*bucketName, *region, *accessKey, *secretKey)
		if err != nil {
			fmt.Printf("%+v", err)
			os.Exit(1)
		}

		// Generate a slice of random object keys
		var keys []string
		for i := 0; i < 5; i++ {
			id := uuid.New()
			keys = append(keys, fmt.Sprintf("foo/%s/%s", id, id))
		}
		// Basic test using Loki object client to put dummy data and delete it after a brief pause
		if err := putObjectBatch(client, keys); err != nil {
			fmt.Printf("%+v", err)
			os.Exit(1)
		}
		time.Sleep(15 * time.Second)
		if err := deleteObjectBatch(client, keys); err != nil {
			fmt.Printf("%+v", err)
			os.Exit(1)
		}
	}()

	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(*addr, nil))
}
