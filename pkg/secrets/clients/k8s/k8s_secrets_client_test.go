package k8s

import (
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log/messages"
)

func TestKubernetesSecrets(t *testing.T) {
	Convey("generateStringDataEntry", t, func() {

		Convey("Given a map of data entries", func() {
			dataEntries := make(map[string][]byte)
			dataEntries["user"] = []byte("dummy_user")
			dataEntries["password"] = []byte("dummy_password")
			dataEntries["address"] = []byte("dummy_address")
			stringDataEntry, err := generateStringDataEntry(dataEntries)

			Convey("Finishes without raising an error", func() {
				So(err, ShouldEqual, nil)
			})

			Convey("Convert the data entry map to a stringData entry in the form of a comma separate byte array", func() {
				stringDataEntryExpected := `{"stringData":{"user":"dummy_user","password":"dummy_password","address":"dummy_address"}}`
				stringDataEntryActual := string(stringDataEntry)
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

		Convey("Given a map of data entries and a secret with backslashes", func() {
			dataEntries := make(map[string][]byte)
			// simulates a Conjur secret with 1 backslash with unicode
			dataEntries["user"] = []byte("super\u005csecret");
			escapedDataEntry, err := generateStringDataEntry(dataEntries);

			Convey("Finishes without raising an error", func() {
				So(err, ShouldEqual, nil)
			})

			Convey("Returns proper password with escaped characters", func() {
				// returned 2x backslashes because k8s trims the second backslash
				// to mimic this, we are expecting two backslashes
				stringDataEntryExpected := `{"stringData":{"user":"super\\secret"}}`
				stringDataEntryActual := string(escapedDataEntry)
				eq := reflect.DeepEqual(stringDataEntryActual, stringDataEntryExpected)
				So(eq, ShouldEqual, true)
			})
		})

		// a more complex use-case where the secret contains a combination of backslashes and the unicode escape sequence for backslashes
		Convey("Given a map of data entries and a secret with unicode and backslashes", func() {
			dataEntries := make(map[string][]byte)
			// simulates a Conjur secret with 1 backslash with unicode
			dataEntries["user"] = []byte("\u005c\u005ca\\u005c\u005cb\u005cc\u005c");
			escapedDataEntry, err := generateStringDataEntry(dataEntries);

			Convey("Finishes without raising an error", func() {
				So(err, ShouldEqual, nil)
			})

			Convey("Returns proper password with escaped characters", func() {
				// before sending a secret to K8s, each secret will be escaped with an additional backslash
				// K8s trims the second backslash so in order to mimic this, we are expecting two backslashes
				stringDataEntryExpected := `{"stringData":{"user":"\\\\a\\u005c\\b\\c\\"}}`
				stringDataEntryActual := string(escapedDataEntry)
				eq := reflect.DeepEqual(stringDataEntryActual, stringDataEntryExpected)
				So(eq, ShouldEqual, true)
			})
		})

		Convey("Given an empty map of data entries", func() {
			dataEntries := make(map[string][]byte)

			Convey("Raises an error that the map input should not be empty", func() {
				_, err := generateStringDataEntry(dataEntries)
				So(err.Error(), ShouldEqual, messages.CSPFK026E)
			})
		})
	})
}
