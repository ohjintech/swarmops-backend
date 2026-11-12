package topics

import "testing"

func TestParse(t *testing.T) {
	f, d, k, ok := Parse("swarm/v1/fleet-1/d-001/telemetry")
	if !ok || f != "fleet-1" || d != "d-001" || k != "telemetry" {
		t.Fatalf("got %q %q %q %v", f, d, k, ok)
	}
	for _, bad := range []string{"", "swarm/v2/a/b/c", "swarm/v1/a/b", "x/v1/a/b/c"} {
		if _, _, _, ok := Parse(bad); ok {
			t.Errorf("Parse(%q) should fail", bad)
		}
	}
}

func TestCommandRoundTrip(t *testing.T) {
	f, d, k, ok := Parse(Command("f", "d"))
	if !ok || f != "f" || d != "d" || k != "cmd" {
		t.Fatal("round trip failed")
	}
}
