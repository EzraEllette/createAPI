package s3Creator

import (
	"autoLambda/eachFile"
	"autoLambda/handleErrors"
	"bytes"
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/google/uuid"
)

type s3Client struct {
	s3          *s3.Client
	BucketName  *string `json:"bucketName"`
	MultpartKey *string `json:"multipartKey"`
	UploadId    *string `json:"uploadId"`
}

var uploads map[string][]types.CompletedPart

var client s3Client

var uploadPart func(path string, file []byte) (uploadPart *s3.UploadPartOutput, err error)

func init() {
	client = s3Client{}
	uploadPart = createUploader()
	uploads = map[string][]types.CompletedPart{}
}

func createBucket() error {
	client.BucketName = aws.String("autoLambda-test-bucket-" + uuid.NewString())
	_, err := client.s3.CreateBucket(context.TODO(), &s3.CreateBucketInput{
		Bucket:                    client.BucketName,
		CreateBucketConfiguration: &types.CreateBucketConfiguration{},
		GrantRead:                 aws.String(string(types.PermissionRead)),
	})

	return err
}

func createUploader() func(path string, file []byte) (uploadPart *s3.UploadPartOutput, err error) {
	var partNumber int32 = 1

	return func(path string, file []byte) (uploaded *s3.UploadPartOutput, err error) {
		body := bytes.NewReader(file)

		uploaded, err = client.s3.UploadPart(context.TODO(), &s3.UploadPartInput{
			Bucket:     client.BucketName,
			Key:        client.MultpartKey,
			PartNumber: partNumber,
			UploadId:   client.UploadId,
			Body:       body,
		})
		uploads[*client.UploadId] = append(uploads[*client.UploadId], types.CompletedPart{
			ETag:       uploaded.ETag,
			PartNumber: partNumber,
		})
		partNumber++
		return
	}
}

func NewBucket(cfg aws.Config, folder string) (err error) {
	client.s3 = s3.NewFromConfig(cfg)

	createBucket()
	client.MultpartKey = aws.String("build")
	upload, err := client.s3.CreateMultipartUpload(context.TODO(), &s3.CreateMultipartUploadInput{
		Bucket:    client.BucketName,
		Key:       client.MultpartKey,
		GrantRead: aws.String("READ"),
	})
	handleErrors.Check(err)

	client.UploadId = upload.UploadId

	eachFile.Recursive("build", func(filename string, file []byte) {
		_, er := uploadPart(filename, file)
		if er != nil {
			err = er
		}
	})

	client.s3.CompleteMultipartUpload(context.TODO(), &s3.CompleteMultipartUploadInput{
		Key:      client.MultpartKey,
		Bucket:   client.BucketName,
		UploadId: client.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: uploads[*client.UploadId],
		},
	})

	return
}
