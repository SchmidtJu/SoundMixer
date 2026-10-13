package scale

import "testing"

func TestLevelEnds(t *testing.T) {
	if got := Level(0, false); got != 0 {
		t.Fatalf("0 = %v", got)
	}
	if got := Level(DeadZone, false); got != 0 {
		t.Fatalf("dead low = %v", got)
	}
	if got := Level(Max, false); got != 1 {
		t.Fatalf("max = %v", got)
	}
	if got := Level(Max-DeadZone, false); got != 1 {
		t.Fatalf("dead high = %v", got)
	}
}

func TestLevelInvert(t *testing.T) {
	if got := Level(0, true); got != 1 {
		t.Fatalf("invert low = %v", got)
	}
	if got := Level(Max, true); got != 0 {
		t.Fatalf("invert high = %v", got)
	}
}

func TestLevelMid(t *testing.T) {
	got := Level((Max)/2, false)
	if got < 0.45 || got > 0.55 {
		t.Fatalf("mid = %v", got)
	}
}

func TestLevelClamp(t *testing.T) {
	if got := Level(-10, false); got != 0 {
		t.Fatalf("neg = %v", got)
	}
	if got := Level(5000, false); got != 1 {
		t.Fatalf("high = %v", got)
	}
}
