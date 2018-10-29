// main.go
package main

import (
	"archive/zip"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudformation"
	"github.com/aws/aws-sdk-go/service/codepipeline"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

const localZippedArtifactPath = "/tmp/merged.zip"
const localUnzipedArtifactPath = "/tmp/merged.yml"

type BucketInfo struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

func handler(artifact events.CodePipelineEvent) error {
	sess := session.Must(session.NewSession())
	defer func() {
		if r := recover(); r != nil {
			fail(artifact, sess)
		}
	}()
	downloadArtifacts(artifact)
	validatecf(sess)
	success(artifact, sess)
	return nil
}

func main() {
	lambda.Start(handler)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func bucket(artifact events.CodePipelineEvent) *BucketInfo {
	s3location := artifact.CodePipelineJob.Data.InputArtifacts[0].Location.S3Location
	return &BucketInfo{
		Bucket: s3location.BucketName,
		Key:    s3location.ObjectKey,
	}
}

func fail(artifact events.CodePipelineEvent, sess *session.Session) {
	log.Println("Sending fail signal to Code Pipeline")

	cp := codepipeline.New(sess)
	input := &codepipeline.PutJobFailureResultInput{
		JobId: aws.String(artifact.CodePipelineJob.ID),
	}
	out, err := cp.PutJobFailureResult(input)
	check(err)
	log.Printf("Code Pipeline fail signal output: %v \n", out)
}

func success(artifact events.CodePipelineEvent, sess *session.Session) {
	log.Println("Sending success signal to Code Pipeline")
	cp := codepipeline.New(sess)
	input := &codepipeline.PutJobSuccessResultInput{
		JobId: aws.String(artifact.CodePipelineJob.ID),
	}
	out, err := cp.PutJobSuccessResult(input)
	check(err)
	log.Printf("Code Pipeline success signal output: %v \n", out)
}

func creds(artifact events.CodePipelineEvent) *credentials.Credentials {
	creds := artifact.CodePipelineJob.Data.ArtifactCredentials
	return credentials.NewStaticCredentials(creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken)
}

func validatecf(sess *session.Session) {

	template, err := ioutil.ReadFile(localUnzipedArtifactPath)
	check(err)

	log.Println("Validating cloud formation template")
	cf := cloudformation.New(sess)
	input := &cloudformation.ValidateTemplateInput{
		TemplateBody: aws.String(string(template)),
	}
	output, err := cf.ValidateTemplate(input)
	check(err)
	log.Printf("%v", output)
	log.Println("Done validating")
}

func downloadArtifacts(artifact events.CodePipelineEvent) {
	bucketInfo := bucket(artifact)
	sess := session.Must(session.NewSession(aws.NewConfig().WithCredentials(creds(artifact))))
	s3Svc := s3.New(sess)

	log.Println("Downloading cloud formation template artifacts")
	downloader := s3manager.NewDownloaderWithClient(s3Svc)
	f, err := os.Create(localZippedArtifactPath)
	check(err)
	defer f.Close()
	_, err = downloader.Download(f, &s3.GetObjectInput{
		Bucket: aws.String(bucketInfo.Bucket),
		Key:    aws.String(bucketInfo.Key),
	})
	check(err)
	check(unzip(localZippedArtifactPath, "/tmp"))
}

func unzip(archive, target string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(target, 0755); err != nil {
		return err
	}

	for _, file := range reader.File {
		path := filepath.Join(target, file.Name)
		if file.FileInfo().IsDir() {
			os.MkdirAll(path, file.Mode())
			continue
		}

		fileReader, err := file.Open()
		if err != nil {
			return err
		}
		defer fileReader.Close()

		targetFile, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return err
		}
		defer targetFile.Close()

		if _, err := io.Copy(targetFile, fileReader); err != nil {
			return err
		}
	}

	return nil
}
