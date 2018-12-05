package vfsftp

import (
	"fmt"
	"os"
	"reflect"
	"testing"
)

func TestConnect(t *testing.T) {
	dp, err := Connect(os.Getenv("TEST_SERVER"), os.Getenv("TEST_SERVER_LOGIN"), os.Getenv("TEST_SERVER_PWD"), "")
	if err != nil {
		t.Fatal(err)
	}

	cts := &CTS{}
	cts.All()

	result := cts.Run(dp)
	fmt.Printf("\n\n%v\n\n", result.String())
	for _, check := range result {
		if check.Result != nil {
			t.Fatal(check.Check.Name, "failed:", reflect.TypeOf(check.Result), ":", check.Result)
		}
	}

}
