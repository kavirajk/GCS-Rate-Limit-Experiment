package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	loki_aws "github.com/grafana/loki/pkg/storage/chunk/aws"
)

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

func testPutAndDeleteBatchOfObjects() error {
	// bucketName, region, accessKey, secret
	client, err := createS3ObjectClient(os.Args[1], os.Args[2], os.Args[3], os.Args[4])
	if err != nil {
		return err
	}

	// Generate a slice of random object keys
	var keys []string
	for i := 0; i < 5; i++ {
		id := uuid.New()
		keys = append(keys, fmt.Sprintf("foo/%s/%s", id, id))
	}

	for _, key := range keys {
		// Dummy data
		err = client.PutObject(context.Background(), key, bytes.NewReader([]byte("hi")))
		if err != nil {
			return err
		}
	}

	time.Sleep(15 * time.Second)

	for _, key := range keys {
		err = client.DeleteObject(context.Background(), key)
		if err != nil {
			return err
		}
	}

	return nil
}

func main() {
	// Basic test using Loki object client to put dummy data and delete it after a brief pause
	err := testPutAndDeleteBatchOfObjects()
	if err != nil {
		fmt.Printf("%+v", err)
		os.Exit(1)
	}
}
