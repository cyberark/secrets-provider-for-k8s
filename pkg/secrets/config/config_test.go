package config

import (
	"fmt"
	"os"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
)

func TestConfig(t *testing.T) {
	Convey("NewFromEnv", t, func() {
		testNamespace := "test-namespace"
		testK8sSecrets := "test-k8s-secret,other-k8s-secret, k8s-secret-after-a-space"
		// remove spaces and split by comma
		testK8sSecretsList := strings.Split(strings.ReplaceAll(testK8sSecrets, " ", ""), ",")

		_ = os.Setenv("MY_POD_NAMESPACE", testNamespace)
		_ = os.Setenv("K8S_SECRETS", testK8sSecrets)

		Convey("Required environment variables exist", func() {
			Convey("Valid value of SECRETS_DESTINATION", func() {
				validStoreTypes := []string{K8S}
				for _, storeType := range validStoreTypes {
					_ = os.Setenv("SECRETS_DESTINATION", storeType)

					config, err := NewFromEnv()
					Convey("Doesn't raise an error", func() {
						So(err, ShouldBeNil)
					})

					expectedConfig := &Config{
						PodNamespace:       testNamespace,
						RequiredK8sSecrets: testK8sSecretsList,
						StoreType:          storeType,
					}

					Convey("Returns the expected config", func() {
						So(config, ShouldResemble, expectedConfig)
					})
				}
			})

			Convey("Invalid value of SECRETS_DESTINATION", func() {
				_ = os.Setenv("SECRETS_DESTINATION", "invalid_store_type")

				_, err := NewFromEnv()
				Convey("Raises the proper error", func() {
					So(err.Error(), ShouldEqual, fmt.Sprintf(messages.CSPFK005E, "SECRETS_DESTINATION"))
				})
			})
		})

		Convey("Missing environment variable", func() {
			requiredEnvVars := []string{"MY_POD_NAMESPACE", "K8S_SECRETS", "SECRETS_DESTINATION"}
			for _, requiredEnvVar := range requiredEnvVars {
				Convey(requiredEnvVar, func() {
					_ = os.Unsetenv(requiredEnvVar)

					config, err := NewFromEnv()
					Convey("Raises the proper error", func() {
						So(config, ShouldBeNil)
						So(err.Error(), ShouldEqual, fmt.Sprintf(messages.CSPFK004E, requiredEnvVar))
					})
				})
			}
		})
	})
}
