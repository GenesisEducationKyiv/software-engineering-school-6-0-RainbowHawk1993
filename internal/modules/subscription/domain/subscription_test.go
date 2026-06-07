package domain

import "testing"

func TestSubscription_NeedsNotification(t *testing.T) {
	sub := Subscription{LastSeenTag: "v1.0.0"}

	if !sub.NeedsNotification("v1.0.1") {
		t.Error("expected true for new version")
	}
	if sub.NeedsNotification("v1.0.0") {
		t.Error("expected false for same version")
	}
}
