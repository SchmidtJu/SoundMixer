package scale

// Max is the highest raw value sent by the firmware.
const Max = 1023

// DeadZone drops the top and bottom of the range.
// With 3.3 V pots the ESP32-S3 ADC does not reach a clean 0 and 1023.
const DeadZone = 31

// Epsilon is the smallest volume change forwarded to Windows.
// The firmware already filters noise. This only skips duplicate reports.
const Epsilon = 0.01

// Level maps a raw value in 0..1023 onto 0..1.
// invert swaps the direction, so the low end is loud and the high end is quiet.
func Level(raw int, invert bool) float32 {
	if raw < 0 {
		raw = 0
	}
	if raw > Max {
		raw = Max
	}

	span := Max - 2*DeadZone
	var level float32
	switch {
	case raw <= DeadZone:
		level = 0
	case raw >= Max-DeadZone:
		level = 1
	default:
		level = float32(raw-DeadZone) / float32(span)
	}
	if invert {
		level = 1 - level
	}
	return level
}
