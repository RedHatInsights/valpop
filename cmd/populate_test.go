package cmd

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

func TestPopulateCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Populate Command Suite")
}

var _ = Describe("Populate Command", func() {
	Describe("CLI flags", func() {
		BeforeEach(func() {
			// Reset viper before each test
			viper.Reset()
		})

		Context("cache-max-age flag", func() {
			It("should have default value of 86400 seconds (1 day)", func() {
				flag := populateCmd.Flags().Lookup("cache-max-age")
				Expect(flag).NotTo(BeNil())
				Expect(flag.DefValue).To(Equal("86400"))
			})

			DescribeTable("should accept various cache-max-age values",
				func(value string, expected int64) {
					err := populateCmd.Flags().Set("cache-max-age", value)
					Expect(err).NotTo(HaveOccurred())

					viper.BindPFlag("cache-max-age", populateCmd.Flags().Lookup("cache-max-age"))
					Expect(viper.GetInt64("cache-max-age")).To(Equal(expected))
				},
				Entry("custom value", "300", int64(300)),
				Entry("zero (no-cache equivalent)", "0", int64(0)),
				Entry("large value (1 year)", "31536000", int64(31536000)),
			)

			It("should have short flag -g", func() {
				flag := populateCmd.Flags().ShorthandLookup("g")
				Expect(flag).NotTo(BeNil())
				Expect(flag.Name).To(Equal("cache-max-age"))
			})

			It("should not conflict with root password shorthand", func() {
				passwordFlag := rootCmd.PersistentFlags().ShorthandLookup("c")
				cacheMaxAgeFlag := populateCmd.Flags().ShorthandLookup("g")

				Expect(passwordFlag).NotTo(BeNil())
				Expect(passwordFlag.Name).To(Equal("password"))
				Expect(cacheMaxAgeFlag).NotTo(BeNil())
				Expect(cacheMaxAgeFlag.Name).To(Equal("cache-max-age"))
			})

			It("should have correct usage description", func() {
				flag := populateCmd.Flags().Lookup("cache-max-age")
				Expect(flag.Usage).To(ContainSubstring("Cache-Control"))
				Expect(flag.Usage).To(ContainSubstring("max-age"))
				Expect(flag.Usage).To(ContainSubstring("seconds"))
			})
		})

		Context("required flag validation", func() {
			It("should require source flag", func() {
				viper.Set("prefix", "test")
				viper.Set("image", "test-image:v1")
				viper.Set("source", "")

				err := populateCmd.RunE(populateCmd, []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no source arg set"))
			})

			It("should require prefix flag", func() {
				viper.Set("source", "/tmp/test")
				viper.Set("image", "test-image:v1")
				viper.Set("prefix", "")

				err := populateCmd.RunE(populateCmd, []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no prefix arg set"))
			})

			It("should validate min-asset-records is non-negative", func() {
				viper.Set("source", "/tmp/test")
				viper.Set("prefix", "test")
				viper.Set("image", "test-image:v1")
				viper.Set("min-asset-records", -1)

				err := populateCmd.RunE(populateCmd, []string{})
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("min-asset-records must be a non-negative integer"))
			})
		})
	})
})
