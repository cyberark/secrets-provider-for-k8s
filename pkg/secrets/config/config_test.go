package config

import (
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"github.com/cyberark/cyberark-secrets-provider-for-k8s/pkg/log/messages"
)

func TestConfig(t *testing.T) {
	Convey("getStoreType", t, func() {
		Convey("Given an incorrect value for SECRETS_DESTINATION env variable", func() {
			_, err := getStoreType("incorrect_secrets_destination")

			Convey("Raises the proper error", func() {
				So(err.Error(), ShouldEqual, fmt.Sprintf(messages.CSPFK005E, "SECRETS_DESTINATION"))
			})
		})
	})
}
