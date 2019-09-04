package k8s

import (
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log"
)

func TestKubernetesSecrets(t *testing.T) {
	Convey("generateStringDataEntry", t, func() {

		Convey("Given a map of data entries", func() {
			m := make(map[string][]byte)
			m["user"] = []byte("dummy_user")
			m["password"] = []byte("dummy_password")
			m["address"] = []byte("dummy_address")
			DataEntry, err := generateStringDataEntry(m)

			Convey("Finishes without raising an error", func() {
				So(err, ShouldEqual, nil)
			})

			Convey("Convert the data entry map to a stringData entry in the form of a comma separate byte array", func() {
				stringDataEntryExpected := `{"stringData":{"user":"dummy_user","password":"dummy_password","address":"dummy_address"}}`
				stringDataEntryActual := string(DataEntry)
				// Sort actual and expected, because output order can change
				re := regexp.MustCompile("\\:{(.*?)\\}")
				// Regex example: {"stringData":{"user":"dummy_user","password":"dummy_password"}} => {"user":"dummy_user","password":"dummy_password"}
				match := re.FindStringSubmatch(stringDataEntryActual)
				stringDataEntryActualSorted := strings.Split(match[1], ",")
				sort.Strings(stringDataEntryActualSorted)
				match = re.FindStringSubmatch(stringDataEntryExpected)
				stringDataEntryExpectedSorted := strings.Split(match[1], ",")
				sort.Strings(stringDataEntryExpectedSorted)
				eq := reflect.DeepEqual(stringDataEntryActualSorted, stringDataEntryExpectedSorted)
				So(eq, ShouldEqual, true)
			})
		})

		Convey("Given an empty map of data entries", func() {
			m := make(map[string][]byte)

			Convey("Raises an error that the map input should not be empty", func() {
				_, err := generateStringDataEntry(m)
				So(err.Error(), ShouldEqual, log.CSPFK039E)
			})
		})
	})
}
