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
		testRetryCountLimit := 5
		testRetryIntervalSec := 1
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
						RetryCountLimit:    testRetryCountLimit,
						RequiredK8sSecrets: testK8sSecretsList,
						RetryIntervalSec:   testRetryIntervalSec,
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

	Convey("NewFromAnnotations", t, func() {
		Convey("When \"conjur.org/secrets-destination\" is not set", func() {
			annotations := map[string]string{
				"conjur.org/k8s-secrets":        `- k8s-secret-1\n- k8s-secret-2\n`,
				"conjur.org/retry-count-limit":  "10",
				"conjur.org/retry-interval-sec": "5",
			}

			_, err := NewFromAnnotations(annotations)

			Convey("Raises the proper error", func() {
				So(err.Error(), ShouldEqual, fmt.Sprintf(messages.CSPFK044E, "conjur.org/secrets-destination"))
			})
		})

		Convey("When \"conjur.org/secrets-destination\" is set to \"file\"", func() {
			Convey("When no other annotations are provided", func() {
				annotations := map[string]string{
					"conjur.org/secrets-destination": "file",
				}

				config, err := NewFromAnnotations(annotations)
				Convey("Don't raise an error", func() {
					So(err, ShouldBeNil)
				})

				Convey("Config field values provide defaults", func() {
					expectedConfig := &Config{
						PodNamespace:       "",
						RequiredK8sSecrets: []string{},
						RetryCountLimit:    5,
						RetryIntervalSec:   1,
						StoreType:          "file",
					}

					So(config, ShouldResemble, expectedConfig)
				})
			})

			Convey("When other annotations are provided", func() {
				annotations := map[string]string{
					"conjur.org/secrets-destination": "file",
					"conjur.org/k8s-secrets":         `- k8s-secret-1\n- k8s-secret-2\n`,
					"conjur.org/retry-count-limit":   "10",
					"conjur.org/retry-interval-sec":  "5",
				}

				config, err := NewFromAnnotations(annotations)
				Convey("Don't raise an error", func() {
					So(err, ShouldBeNil)
				})

				Convey("Returns the expected config", func() {
					expectedConfig := &Config{
						PodNamespace:       "",
						RequiredK8sSecrets: []string{},
						RetryCountLimit:    10,
						RetryIntervalSec:   5,
						StoreType:          "file",
					}

					So(config, ShouldResemble, expectedConfig)
				})
			})
		})

		Convey("When \"conjur.org/secrets-destination\" is set to \"k8s_secrets\"", func() {
			Convey("When no other annotations are provided", func() {
				annotations := map[string]string{
					"conjur.org/secrets-destination": "k8s_secrets",
				}

				config, err := NewFromAnnotations(annotations)
				Convey("Don't raise an error", func() {
					So(err, ShouldBeNil)
				})

				Convey("Config field values indicate that EnvVar settings should be used", func() {
					expectedConfig := &Config{
						PodNamespace:       "",
						RequiredK8sSecrets: nil,
						RetryCountLimit:    -1,
						RetryIntervalSec:   -1,
						StoreType:          "k8s_secrets",
					}

					So(config, ShouldResemble, expectedConfig)
				})
			})

			Convey("When other annotations are provided", func() {
				annotations := map[string]string{
					"conjur.org/secrets-destination": "k8s_secrets",
					"conjur.org/k8s-secrets":         `- k8s-secret-1\n- k8s-secret-2\n`,
					"conjur.org/retry-count-limit":   "10",
					"conjur.org/retry-interval-sec":  "5",
				}

				config, err := NewFromAnnotations(annotations)
				Convey("Don't raise an error", func() {
					So(err, ShouldBeNil)
				})

				Convey("Returns the expected config", func() {
					expectedConfig := &Config{
						PodNamespace:       "",
						RequiredK8sSecrets: []string{"k8s-secret-1", "k8s-secret-2"},
						RetryCountLimit:    10,
						RetryIntervalSec:   5,
						StoreType:          "k8s_secrets",
					}

					So(config, ShouldResemble, expectedConfig)
				})
			})
		})
	})
}
