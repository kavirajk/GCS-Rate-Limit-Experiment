package main

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/google/uuid"
)

func uploadTest(sess *session.Session, bucket string, fileName string) error {
	id := uuid.New()
	key := fmt.Sprintf("foo/%s/%s", id, id)

	file, err := os.Open("./foo")
	if err != nil {
		return fmt.Errorf("error: %v", err)
	}
	defer file.Close()

	uploader := s3manager.NewUploader(sess)
	output, err := uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   file,
	})
	if err != nil {
		return fmt.Errorf("error: %v", err)
	}

	fmt.Printf("Upload completed: \nFile: %s \nBucket: %s \nOutput: %v", fileName, bucket, output)
	return nil
}

func main() {
	bucketName := "dev-us-east-0-loki-rate-limit-test"

	sess, err := session.NewSession(&aws.Config{
		Region: aws.String("us-east-2"),
	})
	if err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}

	err = uploadTest(sess, bucketName, "./foo")
	if err != nil {
		fmt.Printf("%v", err)
		os.Exit(1)
	}
}
