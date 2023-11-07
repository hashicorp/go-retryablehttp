package main

import (
	"crypto/rand"
	"fmt"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"io"
	"log"
	"os/exec"

	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/joho/godotenv"
)

func main() {

	if err := godotenv.Load(); err != nil {
		log.Fatal("Error loading .env file")
	}
	// Initialize a retryable client
	standardClient := retryablehttp.NewClient()

	// Generate a file and get the digest (file name in this context)
	fileDigest := generateFile()

	// Generate a signed URL for PUT request (Upload)
	putURL := getSignedURL(fileDigest, "put")
	fmt.Println("PUT URL:", putURL)

	// Perform the PUT request (Upload)
	err := putS3Object(standardClient, putURL, fileDigest)
	//curlCommand := fmt.Sprintf(`curl --sslv3 --http1.1 -v -T %s -H "Content-Type: application/octet-stream" %s`, fileDigest, putURL)
	//err := putS3ObjectWithCurl(curlCommand)
	if err != nil {
		fmt.Println("Error uploading file:", err)
		return
	}
	fmt.Println("File uploaded successfully.")

	// Generate a signed URL for GET request (Download)
	getURL := getSignedURL(fileDigest, "get")
	fmt.Println("GET URL:", getURL)

	// Perform the GET request (Download)
	err = getS3Object(standardClient, getURL, fileDigest+"_downloaded")
	if err != nil {
		fmt.Println("Error downloading file:", err)
		return
	}
	fmt.Println("File downloaded successfully.")
}

func generateFile() string {
	GB := int64(1024 * 1024 * 100) // 100 Mb
	alias := "100MB"

	fileDigest, err := genFakeFiles(".", GB, fmt.Sprintf("test_"+alias+"_"+strconv.Itoa(int(GB))+"_"+strconv.Itoa(int(time.Now().UnixNano()))))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return fileDigest
}

func genFakeFiles(dir string, size int64, name string) (string, error) {
	filePath := filepath.Join(dir, name)
	file, err := os.Create(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	_, err = io.CopyN(file, rand.Reader, size)
	return filePath, err
}

func getSignedURL(fileDigest, method string) string {

	var bucketName = os.Getenv("BUCKET_NAME")
	var accountId = os.Getenv("ACCOUNT_ID")
	var accessKeyId = os.Getenv("ACCESS_KEY_ID")
	var accessKeySecret = os.Getenv("ACCESS_KEY_SECRET")

	r2Resolver := aws.EndpointResolverWithOptionsFunc(func(service, region string, options ...interface{}) (aws.Endpoint, error) {
		return aws.Endpoint{
			URL: fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountId),
		}, nil
	})

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithEndpointResolverWithOptions(r2Resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyId, accessKeySecret, "")),
	)
	if err != nil {
		log.Fatal(err)
	}

	svc := s3.NewFromConfig(cfg)

	psClient := s3.NewPresignClient(svc)

	var presignedRequest *v4.PresignedHTTPRequest

	switch method {
	case "put":
		presignedRequest, _ = psClient.PresignPutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: &bucketName,
			Key:    &fileDigest,
		}, s3.WithPresignExpires(15*time.Minute)) // URL valid for 15 minutes

	case "get":
		presignedRequest, _ = psClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{
			Bucket: &bucketName,
			Key:    &fileDigest,
		}, s3.WithPresignExpires(15*time.Minute)) // URL valid for 15 minutes
	default:
		fmt.Printf("Unknown method: %s\n", method)
		return ""
	}

	if err != nil {
		fmt.Printf("Failed to sign request for %s: %v\n", method, err)
		return ""
	}

	return presignedRequest.URL
}

func putS3Object(client *retryablehttp.Client, url string, filePath string) error {

	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get the size of the file
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	req, err := retryablehttp.NewRequest(http.MethodPut, url, file)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	//	req.Header.Set("Content-Type", "application/octet-stream")
	req.ContentLength = fileInfo.Size()

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	return nil
}

func getS3Object(client *retryablehttp.Client, url string, targetPath string) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func putS3ObjectWithCurl(curlCommand string) error {
	// Execute the curl command using exec.Command
	fmt.Println("CURL COMMAND:")
	fmt.Println(curlCommand)
	cmd := exec.Command("bash", "-c", curlCommand)

	// Capture the output and error if needed
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("curl command failed: %v\nOutput: %s", err, output)
	}

	// Print the output from the curl command
	fmt.Printf("Output from curl: %s\n", output)
	fmt.Println("cf put done")

	return nil
}
