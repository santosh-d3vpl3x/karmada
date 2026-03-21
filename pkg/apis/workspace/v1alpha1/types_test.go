package v1alpha1

import (
	"reflect"
	"testing"
)

func TestWorkspacePublicContract(t *testing.T) {
	if LiveSubresourcePortForward != "portforward" {
		t.Fatalf("got %q", LiveSubresourcePortForward)
	}
	if ResourceKindWorkspace != "Workspace" {
		t.Fatalf("unexpected workspace kind: %s", ResourceKindWorkspace)
	}
	if ResourceKindPlacementView != "PlacementView" {
		t.Fatalf("unexpected placement view kind: %s", ResourceKindPlacementView)
	}
}

func TestNamespacePolicyModes(t *testing.T) {
	want := []NamespacePolicyMode{
		NamespacePolicyModeShared,
		NamespacePolicyModePrefixed,
		NamespacePolicyModeDedicated,
	}
	got := []NamespacePolicyMode{
		NamespacePolicyModeShared,
		NamespacePolicyModePrefixed,
		NamespacePolicyModeDedicated,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}
