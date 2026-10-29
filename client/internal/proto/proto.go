package proto

import (
	"fmt"
	"strconv"
	"strings"
)

const SliderCount = 4

// ParseLine reads a firmware line of the form "1023|640|0|210".
func ParseLine(line string) ([SliderCount]int, error) {
	var out [SliderCount]int
	line = strings.TrimSpace(line)
	if line == "" {
		return out, fmt.Errorf("empty line")
	}
	parts := strings.Split(line, "|")
	if len(parts) != SliderCount {
		return out, fmt.Errorf("expected %d values, got %d", SliderCount, len(parts))
	}
	for i, part := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return out, fmt.Errorf("value %d: %w", i, err)
		}
		if n < 0 || n > 1023 {
			return out, fmt.Errorf("value %d outside 0..1023: %d", i, n)
		}
		out[i] = n
	}
	return out, nil
}
