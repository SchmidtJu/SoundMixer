package proto

import "testing"

func TestParseLine(t *testing.T) {
	got, err := ParseLine("1023|640|0|210\r")
	if err != nil {
		t.Fatal(err)
	}
	want := [SliderCount]int{1023, 640, 0, 210}
	if got != want {
		t.Fatalf("got %v", got)
	}
}

func TestParseLineRejects(t *testing.T) {
	cases := []string{"", "1|2|3", "1|2|3|4|5", "1|2|3|1024", "-1|0|0|0", "a|0|0|0"}
	for _, line := range cases {
		if _, err := ParseLine(line); err == nil {
			t.Fatalf("expected error for %q", line)
		}
	}
}
