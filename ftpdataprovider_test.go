package vfsftp

import (
	"fmt"
	"net/url"
	"os"
	"reflect"
	"testing"
)

func TestConnect(t *testing.T) {
	curl, err := url.Parse(fmt.Sprintf("ftp://%v:%v@%v", os.Getenv("TEST_SERVER_LOGIN"), os.Getenv("TEST_SERVER_PWD"), os.Getenv("TEST_SERVER")))
	if err != nil {
		t.Fatal(err)
	}
	dp, err := Connect(curl)
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
