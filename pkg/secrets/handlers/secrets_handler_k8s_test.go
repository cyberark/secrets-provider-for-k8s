package handlers

import (
	"reflect"
	"sort"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log"
	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/secrets/k8s"
)

func TestSecretsHandlerK8sUseCase(t *testing.T) {
	Convey("getVariableIDsToRetrieve", t, func() {

		Convey("Given a non-empty pathMap", func() {
			pathMap := make(map[string][]string)

			pathMap["account/var_path1"] = []string{"secret1:key1"}
			pathMap["account/var_path2"] = []string{"secret1:key2"}
			variableIDsExpected := []string{"account/var_path1", "account/var_path2"}
			variableIDsActual, err := getVariableIDsToRetrieve(pathMap)

			Convey("Finishes without raising an error", func() {
				So(err, ShouldEqual, nil)
			})

			Convey("Returns variable IDs in the pathMap as expected", func() {
				// Sort actual and expected, because output order can change
				sort.Strings(variableIDsActual)
				sort.Strings(variableIDsExpected)
				eq := reflect.DeepEqual(variableIDsActual, variableIDsExpected)
				So(eq, ShouldEqual, true)
			})
		})

		Convey("Given an empty pathMap", func() {
			pathMap := make(map[string][]string)

			Convey("Raises an error that the map input is empty", func() {
				_, err := getVariableIDsToRetrieve(pathMap)
				So(err.Error(), ShouldEqual, log.CSPFK029E)
			})
		})
	})

	Convey("updateK8sSecretsMapWithConjurSecrets", t, func() {
		Convey("Given one K8s secret with one Conjur secret", func() {
			secret := []byte{'s', 'u', 'p', 'e', 'r'}
			conjurSecrets := make(map[string][]byte)
			conjurSecrets["account:variable:allowed/username"] = secret

			newDataEntriesMap := make(map[string][]byte)
			newDataEntriesMap["username"] = []byte("allowed/username")

			k8sSecretsMap := make(map[string]map[string][]byte)
			k8sSecretsMap["mysecret"] = newDataEntriesMap

			pathMap := make(map[string][]string)
			pathMap["allowed/username"] = []string{"mysecret:username"}

			k8sSecretsStruct := k8s.K8sSecretsMap{k8sSecretsMap, pathMap}
			err := updateK8sSecretsMapWithConjurSecrets(&k8sSecretsStruct, conjurSecrets)

			Convey("Finishes without raising an error", func() {
				So(err, ShouldEqual, nil)
			})

			Convey("Replaces secret variable IDs in k8sSecretsMap with their corresponding secret value", func() {
				eq := reflect.DeepEqual(k8sSecretsStruct.K8sSecrets["mysecret"]["username"], []byte{'s', 'u', 'p', 'e', 'r'})
				So(eq, ShouldEqual, true)
			})
		})

		Convey("Given 2 k8s secrets that need the same Conjur secret", func() {
			secret := []byte{'s', 'u', 'p', 'e', 'r'}
			conjurSecrets := make(map[string][]byte)
			conjurSecrets["account:variable:allowed/username"] = secret

			dataEntriesMap := make(map[string][]byte)
			dataEntriesMap["username"] = []byte("allowed/username")

			k8sSecretsMap := make(map[string]map[string][]byte)
			k8sSecretsMap["secret"] = dataEntriesMap
			k8sSecretsMap["another-secret"] = dataEntriesMap

			pathMap := make(map[string][]string)
			pathMap["allowed/username"] = []string{"secret:username", "another-secret:username"}

			k8sSecretsStruct := k8s.K8sSecretsMap{k8sSecretsMap, pathMap}
			err := updateK8sSecretsMapWithConjurSecrets(&k8sSecretsStruct, conjurSecrets)

			Convey("Finishes without raising an error", func() {
				So(err, ShouldEqual, nil)
			})

			Convey("Replaces both Variable IDs in k8sSecretsMap to their corresponding secret values without errors", func() {
				eq := reflect.DeepEqual(k8sSecretsStruct.K8sSecrets["secret"]["username"], secret)
				So(eq, ShouldEqual, true)

				eq = reflect.DeepEqual(k8sSecretsStruct.K8sSecrets["another-secret"]["username"], secret)
				So(eq, ShouldEqual, true)
			})
		})
	})
}
