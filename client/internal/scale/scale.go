package scale

// Max ist der höchste Rohwert aus der Firmware.
const Max = 1023

// DeadZone schneidet das obere und untere Ende ab.
// Der ESP32-S3-ADC erreicht bei 3,3-V-Potis nicht sauber 0 und 1023.
const DeadZone = 31

// Epsilon ist die kleinste Lautstärkeänderung, die an Windows weitergegeben wird.
// Das Rauschen filtert die Firmware. Hier bleibt nur ein kleiner Schutz vor Doppelmeldungen.
const Epsilon = 0.01

// Level mappt einen Rohwert 0..1023 auf 0..1.
// invert dreht die Richtung (unten laut, oben leise).
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
