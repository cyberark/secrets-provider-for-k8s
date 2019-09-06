package utils

import (
	"reflect"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestByteSliceUtils(t *testing.T) {
	Convey("ByteSlicePrintf", t, func() {

		Convey("Formats a byteSlice according to a template & a separator", func() {
			template := `prefix{%v}suffix`
			separator := "%v"
			byteSlice := []byte("slice")

			byteSliceSprintfExpected := []byte("prefix{slice}suffix")
			byteSlicePrintfActual := ByteSlicePrintf(template, separator, byteSlice)

			eq := reflect.DeepEqual(byteSlicePrintfActual, byteSliceSprintfExpected)
			So(eq, ShouldEqual, true)
		})

		Convey("Formats more than one byteSlice according to a template & a separator", func() {
			template := `prefix{%v}-{%v}suffix`
			separator := "%v"
			byteSliceFirst := []byte("first-slice")
			byteSliceSecond := []byte("second-slice")

			byteSliceSprintfExpected := []byte("prefix{first-slice}-{second-slice}suffix")
			byteSlicePrintfActual := ByteSlicePrintf(template, separator, byteSliceFirst, byteSliceSecond)

			eq := reflect.DeepEqual(byteSlicePrintfActual, byteSliceSprintfExpected)
			So(eq, ShouldEqual, true)
		})
	})
}
