package mock_test

import (
	"fmt"

	"github.com/RedHatInsights/valpop/impl/mock"
	"github.com/RedHatInsights/valpop/impl/s3"
)

// Example showing how to inject a mock S3Client into the S3 implementation
func Example_usingMockS3Client() {
	// Create a mock S3 client
	mockClient := mock.NewS3Client()

	// Inject the mock client into the S3 implementation
	minioService := s3.NewMinioWithClient(mockClient)

	// Now you can use the minioService with the mock client
	// This allows you to test S3 logic without hitting real S3
	_ = minioService

	// Configure mock behavior
	mockClient.Errors["PutObject"] = nil // Simulate success

	fmt.Println("Mock S3 client injected successfully")
	// Output: Mock S3 client injected successfully
}

// Example showing direct use of mock S3Service
func Example_usingMockS3Service() {
	// Create a mock S3 service (higher-level mock)
	mockService := mock.NewS3Service()

	// Configure mock behavior
	mockService.Errors["SetItem"] = nil // Simulate success

	// Use in tests
	err := mockService.SetItem("namespace", "file.txt", "text/plain", "bucket", 123, "content")
	if err != nil {
		fmt.Printf("unexpected error: %v\n", err)
		return
	}

	// Verify behavior
	content, exists := mockService.GetStoredItem("namespace", "file.txt")
	if !exists {
		fmt.Println("item not found")
		return
	}

	fmt.Printf("Content: %s\n", content)
	// Output: Content: content
}
