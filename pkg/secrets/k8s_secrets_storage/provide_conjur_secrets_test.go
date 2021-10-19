package k8s_secrets_storage

import (
	"fmt"
	"sort"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	secretsStorageMocks "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/k8s_secrets_storage/mocks"
	conjurMocks "github.com/cyberark/secrets-provider-for-k8s/pkg/secrets/clients/conjur/mocks"
)

func TestProvideConjurSecrets(t *testing.T) {
	Convey("getVariableIDsToRetrieve", t, func() {

		Convey("Given a non-empty pathMap", func() {
			pathMap := map[string][]string{
				"account/var_path1": {"secret1:key1"},
				"account/var_path2": {"secret1:key2"},
			}
			variableIDsExpected := []string{"account/var_path1", "account/var_path2"}
			variableIDsActual, err := getVariableIDsToRetrieve(pathMap)

			Convey("Finishes without raising an error", func() {
				So(err, ShouldEqual, nil)
			})

			Convey("Returns variable IDs in the pathMap as expected", func() {
				// Sort actual and expected, because output order can change
				sort.Strings(variableIDsActual)
				sort.Strings(variableIDsExpected)

				So(variableIDsActual, ShouldResemble, variableIDsExpected)
			})
		})

		Convey("Given an empty pathMap", func() {
			pathMap := map[string][]string{}

			Convey("Raises an error that the map input is empty", func() {
				_, err := getVariableIDsToRetrieve(pathMap)
				So(err.Error(), ShouldEqual, messages.CSPFK025E)
			})
		})
	})

	Convey("updateK8sSecretsMapWithConjurSecrets", t, func() {
		Convey("Given one K8s secret with one Conjur secret", func() {
			conjurSecrets := map[string][]byte{
				"allowed/username": []byte("super"),
			}

			k8sSecretsMap := map[string]map[string][]byte{
				"mysecret": {
					"username": []byte("allowed/username"),
				},
			}

			pathMap := map[string][]string{
				"allowed/username": {"mysecret:username"},
			}

			k8sSecretsStruct := K8sSecretsMap{k8sSecretsMap, nil, pathMap}
			err := updateK8sSecretsMapWithConjurSecrets(&k8sSecretsStruct, conjurSecrets)

			Convey("Finishes without raising an error", func() {
				So(err, ShouldEqual, nil)
			})

			Convey("Replaces secret variable IDs in k8sSecretsMap with their corresponding secret value", func() {
				So(k8sSecretsStruct.K8sSecrets["mysecret"]["username"], ShouldResemble, []byte("super"))
			})
		})

		Convey("Given 2 k8s secrets that need the same Conjur secret", func() {
			conjurSecrets := map[string][]byte{
				"allowed/username": []byte("super"),
			}
			dataEntriesMap := map[string][]byte{
				"username": []byte("allowed/username"),
			}
			k8sSecretsMap := map[string]map[string][]byte{
				"secret": dataEntriesMap,
				"another-secret": dataEntriesMap,
			}

			pathMap := map[string][]string{
				"allowed/username": {"secret:username", "another-secret:username"},
			}

			k8sSecretsStruct := K8sSecretsMap{k8sSecretsMap, nil, pathMap}
			err := updateK8sSecretsMapWithConjurSecrets(&k8sSecretsStruct, conjurSecrets)

			Convey("Finishes without raising an error", func() {
				So(err, ShouldEqual, nil)
			})

			Convey("Replaces both Variable IDs in k8sSecretsMap to their corresponding secret values without errors", func() {
				secret := []byte("super")
				So(k8sSecretsStruct.K8sSecrets["secret"]["username"], ShouldResemble, secret)
				So(k8sSecretsStruct.K8sSecrets["another-secret"]["username"], ShouldResemble, secret)
			})
		})
	})

	Convey("RetrieveRequiredK8sSecrets", t, func() {
		Convey("Given an existing k8s secret that is mapped to an existing conjur secret", func() {
			kubeMockClient := secretsStorageMocks.NewKubeSecretsMockClient()
			kubeMockClient.AddSecret("k8s_secret1", "secret_key", "conjur_variable1")

			requiredSecrets := []string{"k8s_secret1"}

			k8sSecretsMap, err := RetrieveRequiredK8sSecrets(kubeMockClient.RetrieveSecret, "someNameSpace", requiredSecrets)

			Convey("Finishes without raising an error", func() {
				So(err, ShouldEqual, nil)
			})

			Convey("Creates K8sSecrets map as expected", func() {
				expectedK8sSecrets := map[string]map[string][]byte{
					"k8s_secret1": {
						"secret_key": []byte("conjur_variable1"),
					},
				}

				So(k8sSecretsMap.K8sSecrets, ShouldResemble, expectedK8sSecrets)
			})

			Convey("Creates PathMap map as expected", func() {
				expectedPathMap := map[string][]string{
					"conjur_variable1": {"k8s_secret1:secret_key"},
				}

				So(k8sSecretsMap.PathMap, ShouldResemble, expectedPathMap)
			})
		})

		Convey("Given no 'get' permissions on the 'secrets' k8s resource", func() {
			kubeMockClient := secretsStorageMocks.NewKubeSecretsMockClient()
			kubeMockClient.CanRetrieve = false
			kubeMockClient.AddSecret("k8s_secret1", "secret_key", "conjur_variable1")

			requiredSecrets := []string{"k8s_secret1"}

			_, err := RetrieveRequiredK8sSecrets(kubeMockClient.RetrieveSecret, "someNameSpace", requiredSecrets)

			Convey("Raises proper error", func() {
				So(err.Error(), ShouldEqual, messages.CSPFK020E)
			})
		})

		Convey("Given a non-existing k8s secret", func() {
			kubeMockClient := secretsStorageMocks.NewKubeSecretsMockClient()

			requiredSecrets := []string{"non_existing"}

			_, err := RetrieveRequiredK8sSecrets(kubeMockClient.RetrieveSecret, "someNameSpace", requiredSecrets)

			Convey("Raises proper error", func() {
				So(err.Error(), ShouldEqual, messages.CSPFK020E)
			})
		})

		Convey("Given a k8s secret without 'conjur-map'", func() {
			kubeMockClient := secretsStorageMocks.NewKubeSecretsMockClient()
			kubeMockClient.Database["no_conjur_map_secret"] = map[string][]byte{
				"not-conjur-map": []byte("some-data"),
			}

			requiredSecrets := []string{"no_conjur_map_secret"}
			_, err := RetrieveRequiredK8sSecrets(kubeMockClient.RetrieveSecret, "someNameSpace", requiredSecrets)

			Convey("Raises proper error", func() {
				So(err.Error(), ShouldEqual, fmt.Sprintf(messages.CSPFK028E, "no_conjur_map_secret"))
			})
		})

		Convey("Given a k8s secret with an empty 'conjur-map'", func() {
			kubeMockClient := secretsStorageMocks.NewKubeSecretsMockClient()
			kubeMockClient.Database["empty_conjur_map_secret"] = map[string][]byte{
				"conjur-map": []byte(""),
			}

			requiredSecrets := []string{"empty_conjur_map_secret"}

			_, err := RetrieveRequiredK8sSecrets(kubeMockClient.RetrieveSecret, "someNameSpace", requiredSecrets)

			Convey("Raises proper error", func() {
				So(err.Error(), ShouldEqual, fmt.Sprintf(messages.CSPFK028E, "empty_conjur_map_secret"))
			})
		})

		Convey("Given a k8s secret with an invalid 'conjur-map'", func() {
			kubeMockClient := secretsStorageMocks.NewKubeSecretsMockClient()
			kubeMockClient.Database["invalid_conjur_map_secret"] = map[string][]byte{
				"conjur-map": []byte("key_with_no_value"),
			}

			requiredSecrets := []string{"invalid_conjur_map_secret"}

			_, err := RetrieveRequiredK8sSecrets(kubeMockClient.RetrieveSecret, "someNameSpace", requiredSecrets)

			Convey("Raises proper error", func() {
				So(err.Error(), ShouldEqual, fmt.Sprintf(messages.CSPFK028E, "invalid_conjur_map_secret"))
			})
		})
	})

	Convey("UpdateRequiredK8sSecrets", t, func() {
		Convey("Given no 'update' permissions on the 'secrets' k8s resource", func() {
			kubeMockClient := secretsStorageMocks.NewKubeSecretsMockClient()
			kubeMockClient.CanUpdate = false
			kubeMockClient.AddSecret("k8s_secret1", "secret_key1", "conjur_variable1")
			requiredSecrets := []string{"k8s_secret1"}

			k8sSecretsMap, err := RetrieveRequiredK8sSecrets(kubeMockClient.RetrieveSecret, "someNameSpace", requiredSecrets)

			Convey("Finishes without raising an error", func() {
				So(err, ShouldEqual, nil)
			})

			err = UpdateRequiredK8sSecrets(kubeMockClient.UpdateSecret, "someNameSpace", k8sSecretsMap)

			Convey("Raises proper error", func() {
				So(err.Error(), ShouldEqual, messages.CSPFK022E)
			})
		})
	})

	Convey("run", t, func() {
		var mockAccessToken conjurMocks.MockAccessToken

		Convey("Given 2 k8s secrets that only one is required by the pod", func() {
			conjurMockClient := conjurMocks.NewConjurMockClient()

			kubeMockClient := secretsStorageMocks.NewKubeSecretsMockClient()
			kubeMockClient.AddSecret("k8s_secret1", "secret_key1", "conjur_variable1")
			kubeMockClient.AddSecret("k8s_secret2", "secret_key2", "conjur_variable2")
			requiredSecrets := []string{"k8s_secret1"}

			err := run(
				kubeMockClient.RetrieveSecret,
				kubeMockClient.UpdateSecret,
				"someNameSpace",
				requiredSecrets,
				mockAccessToken,
				conjurMockClient.RetrieveSecrets,
			)

			Convey("Finishes without raising an error", func() {
				So(err, ShouldEqual, nil)
			})

			Convey("Updates K8s secrets with their corresponding Conjur secrets", func() {
				verifyK8sSecretValue(kubeMockClient, "k8s_secret1", "secret_key1", "conjur_secret1")

			})

			Convey("Does not updates other K8s secrets", func() {
				actualK8sSecretDataValue := kubeMockClient.Database["k8s_secret2"]["secretkkey1"]
				So(actualK8sSecretDataValue, ShouldEqual, nil)
			})
		})

		Convey("Given 2 k8s secrets that are required by the pod - each one has its own Conjur secret", func() {
			conjurMockClient := conjurMocks.NewConjurMockClient()

			kubeMockClient := secretsStorageMocks.NewKubeSecretsMockClient()
			kubeMockClient.AddSecret("k8s_secret1","secret_key1","conjur_variable1")
			kubeMockClient.AddSecret("k8s_secret2", "secret_key2","conjur_variable2")
			requiredSecrets := []string{"k8s_secret1", "k8s_secret2"}

			err := run(
				kubeMockClient.RetrieveSecret,
				kubeMockClient.UpdateSecret,
				"someNameSpace",
				requiredSecrets,
				mockAccessToken,
				conjurMockClient.RetrieveSecrets,
			)

			Convey("Finishes without raising an error", func() {
				So(err, ShouldEqual, nil)
			})

			Convey("Updates K8s secrets with their corresponding Conjur secrets", func() {
				verifyK8sSecretValue(kubeMockClient, "k8s_secret1", "secret_key1", "conjur_secret1")
				verifyK8sSecretValue(kubeMockClient, "k8s_secret2", "secret_key2", "conjur_secret2")
			})
		})

		Convey("Given 2 k8s secrets that are required by the pod - both have the same Conjur secret", func() {
			conjurMockClient := conjurMocks.NewConjurMockClient()

			kubeMockClient := secretsStorageMocks.NewKubeSecretsMockClient()
			kubeMockClient.AddSecret("k8s_secret1", "secret_key1", "conjur_variable1")
			kubeMockClient.AddSecret("k8s_secret2", "secret_key2", "conjur_variable2")

			requiredSecrets := []string{"k8s_secret1", "k8s_secret2"}

			err := run(
				kubeMockClient.RetrieveSecret,
				kubeMockClient.UpdateSecret,
				"someNameSpace",
				requiredSecrets,
				mockAccessToken,
				conjurMockClient.RetrieveSecrets,
			)

			Convey("Finishes without raising an error", func() {
				So(err, ShouldEqual, nil)
			})

			Convey("Updates K8s secrets with their corresponding Conjur secrets", func() {
				verifyK8sSecretValue(kubeMockClient, "k8s_secret1", "secret_key1", "conjur_secret1")
				verifyK8sSecretValue(kubeMockClient, "k8s_secret2", "secret_key2", "conjur_secret2")
			})
		})

		Convey("Given a k8s secret which is mapped to a non-existing conjur variable", func() {
			conjurMockClient := conjurMocks.NewConjurMockClient()

			kubeMockClient := secretsStorageMocks.NewKubeSecretsMockClient()
			kubeMockClient.AddSecret("k8s_secret1", "secret_key", "non_existing_conjur_variable")

			requiredSecrets := []string{"k8s_secret1"}

			err := run(
				kubeMockClient.RetrieveSecret,
				kubeMockClient.UpdateSecret,
				"someNameSpace",
				requiredSecrets,
				mockAccessToken,
				conjurMockClient.RetrieveSecrets,
			)

			Convey("Raises proper error", func() {
				So(err.Error(), ShouldEqual, fmt.Sprintf(messages.CSPFK034E, "no_conjur_secret_error"))
			})
		})

		Convey("Given a k8s secret which is mapped to a conjur secret with an empty secret value", func() {
			conjurMockClient := conjurMocks.NewConjurMockClient()

			kubeMockClient := secretsStorageMocks.NewKubeSecretsMockClient()
			kubeMockClient.AddSecret("k8s_secret_with_empty_conjur_variable", "secret_key", "conjur_variable_empty_secret")
			requiredSecrets := []string{"k8s_secret_with_empty_conjur_variable"}

			err := run(
				kubeMockClient.RetrieveSecret,
				kubeMockClient.UpdateSecret,
				"someNameSpace",
				requiredSecrets,
				mockAccessToken,
				conjurMockClient.RetrieveSecrets,
			)

			Convey("Finishes without raising an error", func() {
				So(err, ShouldEqual, nil)
			})

			Convey("Updates K8s secrets with their corresponding Conjur secrets", func() {
				verifyK8sSecretValue(kubeMockClient, "k8s_secret_with_empty_conjur_variable", "secret_key", "")
			})
		})

		Convey("Given no 'execute' permissions on the conjur secret", func() {
			conjurMockClient := conjurMocks.NewConjurMockClient()
			// no execute privileges on the conjur secret
			conjurMockClient.CanExecute = false

			kubeMockClient := secretsStorageMocks.NewKubeSecretsMockClient()
			kubeMockClient.AddSecret("k8s_secret_with_no_permission_conjur_variable", "secret_key", "no_execute_permission_conjur_secret")
			requiredSecrets := []string{"k8s_secret_with_no_permission_conjur_variable"}

			err := run(
				kubeMockClient.RetrieveSecret,
				kubeMockClient.UpdateSecret,
				"someNameSpace",
				requiredSecrets,
				mockAccessToken,
				conjurMockClient.RetrieveSecrets,
			)

			Convey("Raises proper error", func() {
				So(err.Error(), ShouldEqual, fmt.Sprintf(messages.CSPFK034E, "custom error"))
			})
		})
	})
}

func verifyK8sSecretValue(
	client secretsStorageMocks.KubeSecretsMockClient,
	secretName string,
	key string,
	value string,
) {
	actualK8sSecretDataValue := client.Database[secretName][key]
	expectedK8sSecretDataValue := []byte(value)
	So(actualK8sSecretDataValue, ShouldResemble, expectedK8sSecretDataValue)
}
