package main

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	// Import all test suites to run them together
	_ "github.com/RedHatInsights/valpop/cmd"
	_ "github.com/RedHatInsights/valpop/impl"
	_ "github.com/RedHatInsights/valpop/impl/s3"
)

func TestValpop(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Valpop Integration Suite")
}
