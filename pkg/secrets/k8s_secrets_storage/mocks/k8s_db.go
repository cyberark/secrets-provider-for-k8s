package mocks

import "fmt"

// Mocks a K8s database. Maps k8s secret names to mock K8s secrets.
var MockK8sDB map[string]MockK8sSecret

func CreateMockK8sSecret(dataEntries map[string]string) MockK8sSecret {
	secretData := make(map[string][]byte)
	secretData["conjur-map"] = []byte(createConjurMapDataEntry(dataEntries))

	return MockK8sSecret{
		Data: secretData,
	}
}

func createConjurMapDataEntry(dataEntries map[string]string) string {
	// combine each key-value entry to "key:value"
	index := 0
	entriesCombined := make([]string, len(dataEntries))
	for key, value := range dataEntries {
		entry := fmt.Sprintf("\"%s\": \"%s\"", key, value)
		entriesCombined[index] = entry
		index++
	}

	// Add a comma between each pair of entries
	numOfDataEntries := len(dataEntries)
	dataEntriesCombined := entriesCombined[0]
	for i := range entriesCombined {
		if i < numOfDataEntries-1 {
			dataEntriesCombined = fmt.Sprintf("%s, %s", dataEntriesCombined, entriesCombined[i+1])
		}
	}

	// Wrap all the entries
	return fmt.Sprintf("{%s}", dataEntriesCombined)
}
