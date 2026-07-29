package domains

import "testing"

func TestResolvePath(t *testing.T) {
	p, ok := Resolve("/ip/firewall/filter")
	if !ok || p != "/ip/firewall/filter" {
		t.Fatalf("got %q %v", p, ok)
	}
}

func TestResolveAlias(t *testing.T) {
	p, ok := Resolve("firewall/filter")
	if !ok || p != "/ip/firewall/filter" {
		t.Fatalf("got %q %v", p, ok)
	}
	p, ok = Resolve("user")
	if !ok || p != "/user" {
		t.Fatalf("user: %q %v", p, ok)
	}
	p, ok = Resolve("radius")
	if !ok || p != "/radius" {
		t.Fatalf("radius: %q %v", p, ok)
	}
}

func TestJoinFriendly(t *testing.T) {
	if got := JoinFriendly([]string{"firewall", "filter"}); got != "firewall/filter" {
		t.Fatalf("got %q", got)
	}
}

func TestListNonEmpty(t *testing.T) {
	if len(List()) < 10 {
		t.Fatal("expected many aliases")
	}
}
