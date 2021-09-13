package annotations

import (
	"fmt"
	"os"
	"testing"

	"github.com/cyberark/secrets-provider-for-k8s/pkg/log/messages"
	. "github.com/smartystreets/goconvey/convey"
)

var test_annotation_filename string = "test_annots"
var test_annotation_file *os.File
var err error

func clearFileAndWrite(content string) {
	test_annotation_file.Truncate(0)
	test_annotation_file.WriteString(content)
}

func TestAnnotationParser(t *testing.T) {
	Convey("ParseAnnotations", t, func() {

		test_annotation_file, err = os.Create(test_annotation_filename)
		if err != nil {
			t.Errorf("Failed to create sample annotations file for testing: %s\n", err.Error())
		}

		Convey("Given a correctly formatted annotations file", func() {

			valid_file := `conjur.org/authn-identity="host/conjur/authn-k8s/cluster/apps/inventory-api"
conjur.org/container-mode="init"
conjur.org/secrets-destination="k8s-secret"
conjur.org/k8s-secrets="- k8s-secret-1\n- k8s-secret-2\n"
conjur.org/retry-count-limit="10"
conjur.org/retry-interval-sec="5"
conjur.org/debug-logging="true"
conjur.org/conjur-secrets.this-group="- test/url\n- test-password: test/password\n- test-username: test/username\n"
conjur.org/secret-file-path.this-group="this-relative-path"
conjur.org/secret-file-format.this-group="yaml"`

			clearFileAndWrite(valid_file)

			Convey("All annotations will be included in the Map, and their values are maintained", func() {
				annotationsMap, err := ParseAnnotations(test_annotation_filename)
				So(err, ShouldBeNil)

				So(annotationsMap["conjur.org/authn-identity"], ShouldEqual, "host/conjur/authn-k8s/cluster/apps/inventory-api")
				So(annotationsMap["conjur.org/container-mode"], ShouldEqual, "init")
				So(annotationsMap["conjur.org/secrets-destination"], ShouldEqual, "k8s-secret")
				So(annotationsMap["conjur.org/k8s-secrets"], ShouldEqual, `- k8s-secret-1\n- k8s-secret-2\n`)
				So(annotationsMap["conjur.org/retry-count-limit"], ShouldEqual, "10")
				So(annotationsMap["conjur.org/retry-interval-sec"], ShouldEqual, "5")
				So(annotationsMap["conjur.org/debug-logging"], ShouldEqual, "true")
				So(annotationsMap["conjur.org/conjur-secrets.this-group"], ShouldEqual, `- test/url\n- test-password: test/password\n- test-username: test/username\n`)
				So(annotationsMap["conjur.org/secret-file-path.this-group"], ShouldEqual, "this-relative-path")
				So(annotationsMap["conjur.org/secret-file-format.this-group"], ShouldEqual, "yaml")
			})
		})

		Convey("Given an annotations file with allowed values", func() {
			allowed_values := `conjur.org/container-mode="application"
conjur.org/secrets-destination="file"`

			clearFileAndWrite(allowed_values)

			Convey("The annotations will be included in the Map", func() {
				annotationsMap, err := ParseAnnotations(test_annotation_filename)
				So(err, ShouldBeNil)

				So(annotationsMap["conjur.org/container-mode"], ShouldEqual, "application")
				So(annotationsMap["conjur.org/secrets-destination"], ShouldEqual, "file")
			})
		})

		Convey("Given an annotations file with unrecognized keys", func() {
			bad_keys := `conjur.org/valid-but-unrecognized="good-value"
invalid.org/container-mode="init"
sample-bad-key="good-value"`

			clearFileAndWrite(bad_keys)

			Convey("The Map will be empty", func() {
				annotationsMap, err := ParseAnnotations(test_annotation_filename)
				So(err, ShouldBeNil)

				So(annotationsMap, ShouldBeEmpty)
			})
		})

		Convey("Given a \"filepath\" argument that does not exist", func() {
			annotationsMap, err := ParseAnnotations("another_filepath")

			Convey("The function call returns a nil Map and raises the proper error", func() {
				So(annotationsMap, ShouldBeNil)
				So(err.Error(), ShouldEqual, fmt.Sprintf(messages.CSPFK041E, "open another_filepath: no such file or directory"))
			})
		})

		Convey("Given an annotations file with improper value types", func() {
			wrong_value_types_int := `conjur.org/retry-count-limit="seven"`
			wrong_value_type_bool := `conjur.org/debug-logging="not-a-bool"`

			Convey("For annotation requiring an Integer", func() {
				clearFileAndWrite(wrong_value_types_int)

				annotationsMap, err := ParseAnnotations(test_annotation_filename)

				Convey("The function call returns a nil Map and raises the proper error", func() {
					So(annotationsMap, ShouldBeNil)
					So(err.Error(), ShouldEqual, fmt.Sprintf(messages.CSPFK042E, "conjur.org/retry-count-limit", "seven", "Integer"))
				})
			})

			Convey("For annotation requiring a Boolean", func() {
				clearFileAndWrite(wrong_value_type_bool)

				annotationsMap, err := ParseAnnotations(test_annotation_filename)

				Convey("The function call returns a nil Map and raises the proper error", func() {
					So(annotationsMap, ShouldBeNil)
					So(err.Error(), ShouldEqual, fmt.Sprintf(messages.CSPFK042E, "conjur.org/debug-logging", "not-a-bool", "Boolean"))
				})
			})
		})

		Convey("Given an annotations file with a non-allowed value", func() {
			nonallowed_value_container_mode := `conjur.org/container-mode="bad-container-mode"`
			nonallowed_value_secrets_destination := `conjur.org/secrets-destination="bad-secrets-destination"`
			nonallowed_value_secret_file_format := `conjur.org/secret-file-format.this-group="bad-format"`

			Convey("For annotation \"conjur.org/container-mode\"", func() {
				clearFileAndWrite(nonallowed_value_container_mode)

				Convey("The function call returns a nil Map and raises the proper error", func() {
					annotationsMap, err := ParseAnnotations(test_annotation_filename)

					So(annotationsMap, ShouldBeNil)
					So(err.Error(), ShouldEqual, fmt.Sprintf(messages.CSPFK043E, "conjur.org/container-mode", "bad-container-mode", []string{"init", "application"}))
				})
			})

			Convey("For annotation \"conjur.org/secrets-destination\"", func() {
				clearFileAndWrite(nonallowed_value_secrets_destination)

				Convey("The function call returns a nil Map and raises the proper error", func() {
					annotationsMap, err := ParseAnnotations(test_annotation_filename)

					So(annotationsMap, ShouldBeNil)
					So(err.Error(), ShouldEqual, fmt.Sprintf(messages.CSPFK043E, "conjur.org/secrets-destination", "bad-secrets-destination", []string{"file", "k8s-secret"}))
				})
			})

			Convey("For annotations of the form \"conjur.org/secret-file-format.{secret-group}\"", func() {
				clearFileAndWrite(nonallowed_value_secret_file_format)

				Convey("The function call returns a nil Map and raises the proper error", func() {
					annotationsMap, err := ParseAnnotations(test_annotation_filename)

					So(annotationsMap, ShouldBeNil)
					So(err.Error(), ShouldEqual, fmt.Sprintf(messages.CSPFK043E, "conjur.org/secret-file-format.this-group", "bad-format", []string{"yaml", "json", "dotenv", "bash"}))
				})
			})
		})

		err = test_annotation_file.Close()
		if err != nil {
			t.Errorf("Could not close sample annotations file: %s\n", err.Error())
		}
		err = os.Remove(test_annotation_filename)
		if err != nil {
			t.Errorf("Could not remove sample annotation file: %s\n", err.Error())
		}
	})
}
