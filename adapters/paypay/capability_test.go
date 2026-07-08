package paypay

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/monetakit/monetakit/adapterkit"
)

// Capabilities() and capability.json are two declarations of the same fact;
// keep them from drifting.
func TestCapabilityJSONMatchesCode(t *testing.T) {
	raw, err := os.ReadFile("capability.json")
	if err != nil {
		t.Fatal(err)
	}
	var fromFile adapterkit.Capabilities
	if err := json.Unmarshal(raw, &fromFile); err != nil {
		t.Fatal(err)
	}
	if got := New().Capabilities(); !reflect.DeepEqual(got, fromFile) {
		t.Errorf("Capabilities() = %+v, capability.json = %+v", got, fromFile)
	}
}
