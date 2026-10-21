#include <Arduino.h>

// Slider 1..4, links nach rechts, an GP1..GP4.
// Schieberegler: Pin 1 = GND, Pin 2 = GP, Pin 3 = 3V3.
static const int kPins[4] = {1, 2, 3, 4};
static const int kSliderCount = 4;
// Diagnose: GPIO 1..4 sind die ersten vier ADC1-Kanäle, Rohwerte 0..4095.
static const int kGpioLogMs = 500;
static const int kSamples = 9;
static const int kHeartbeatMs = 1000;
// Ruhe 19.10., Rohwerte 0..4095: typisch ±10, selten ±20, einzelne Ausreißer 100..230.
// Nach >>2 und IIR bleibt das Rauschen unter 4. Ein Ausreißer schiebt smooth einmal
// um etwa 15 und hält die Richtung nicht. Darunter bleibt der Wert stehen (~1,2 %).
static const int kIdleBand = 12;
// In Bewegung folgt die Lautstärke in etwa 1-%-Schritten (8 von 961).
static const int kStep = 8;
// Gleiche Richtung, bevor der Stillstand verlassen wird (~100 ms).
static const int kSettle = 8;
// Abstand unter kStep, bevor der Regler wieder als still gilt (~400 ms).
static const int kStill = 30;

struct Track {
  int smooth;
  int locked;
  uint8_t hold;
  uint8_t rest;
  bool moving;
  bool up;
  bool ready;
};

static int median9(int *s) {
  for (int i = 1; i < kSamples; i++) {
    int value = s[i];
    int j = i;
    while (j > 0 && s[j - 1] > value) {
      s[j] = s[j - 1];
      j--;
    }
    s[j] = value;
  }
  return s[kSamples / 2];
}

static int readAdcRaw(int pin) {
  (void)analogRead(pin);
  delayMicroseconds(400);
  return analogRead(pin);
}

static bool sendLine(const char *line, size_t length) {
  // Nur ganze Zeilen einreihen. Bei Rückstau weiter messen statt auf USB warten.
  if (!Serial || Serial.availableForWrite() < (int)length) {
    return false;
  }
  return Serial.write((const uint8_t *)line, length) == length;
}

static bool sendSliders(const Track *trackers) {
  char line[32];
  int length = snprintf(line, sizeof(line), "%d|%d|%d|%d\n", trackers[0].locked,
                        trackers[1].locked, trackers[2].locked, trackers[3].locked);
  return length > 0 && (size_t)length < sizeof(line) && sendLine(line, (size_t)length);
}

static void logGpioInputs() {
  static uint32_t lastMs = 0;
  uint32_t now = millis();
  if (lastMs != 0 && now - lastMs < (uint32_t)kGpioLogMs) {
    return;
  }
  lastMs = now;
  char line[64];
  int length = snprintf(line, sizeof(line), "gpio 1=%d 2=%d 3=%d 4=%d\n", readAdcRaw(1),
                        readAdcRaw(2), readAdcRaw(3), readAdcRaw(4));
  if (length > 0 && (size_t)length < sizeof(line)) {
    sendLine(line, (size_t)length);
  }
}

static int readSlider(int pin) {
  // Erste Messung nach dem Kanalwechsel verwerfen, der ADC braucht Einschwingzeit.
  (void)analogRead(pin);
  delayMicroseconds(400);
  int samples[kSamples];
  for (int i = 0; i < kSamples; i++) {
    samples[i] = analogRead(pin);
    delayMicroseconds(200);
  }
  int scaled = median9(samples) >> 2;  // 12 Bit (0..4095) -> 0..1023
  if (scaled < 0) {
    scaled = 0;
  }
  if (scaled > 1023) {
    scaled = 1023;
  }
  return scaled;
}

void setup() {
  analogReadResolution(12);
  analogSetAttenuation(ADC_11db);
  for (int i = 0; i < kSliderCount; i++) {
    analogSetPinAttenuation(kPins[i], ADC_11db);
  }

  Serial.begin(115200);
  // Null läuft im HWCDC-Wartezähler älterer Cores über. Zusätzlich vor jedem
  // Versand den freien Platz prüfen; der positive Timeout begrenzt den Fehlerfall.
  Serial.setTxTimeoutMs(10);
  delay(200);
  const char banner[] = "gpio pin1-4=adc\n";
  sendLine(banner, sizeof(banner) - 1);
}

static void track(Track &t, int raw, bool &changed) {
  if (!t.ready) {
    t.smooth = raw;
    t.locked = raw;
    t.ready = true;
    changed = true;
    return;
  }

  // Median allein springt noch. Ein kleiner Tiefpass nimmt den einzelnen Zacken weg.
  t.smooth += (raw - t.smooth) / 4;
  int diff = t.smooth - t.locked;
  int dist = diff < 0 ? -diff : diff;
  bool up = diff > 0;

  if (!t.moving) {
    if (dist < kIdleBand) {
      t.hold = 0;
      return;
    }
    if (t.hold == 0 || up == t.up) {
      t.up = up;
      if (t.hold < 255) {
        t.hold++;
      }
    } else {
      t.up = up;
      t.hold = 1;
      return;
    }
    if (t.hold < kSettle) {
      return;
    }
    t.moving = true;
    t.hold = 0;
    t.rest = 0;
    t.locked = t.smooth;
    changed = true;
    return;
  }

  // Langsames Schieben bleibt in diesem Modus. Erst nach längerer Ruhe zurück,
  // sonst verlangt jeder kleine Schritt wieder die grobe Stillstandsschwelle.
  if (dist < kStep) {
    t.hold = 0;
    if (t.rest < 255) {
      t.rest++;
    }
    if (t.rest < kStill) {
      return;
    }
    t.moving = false;
    t.rest = 0;
    if (t.locked != t.smooth) {
      t.locked = t.smooth;
      changed = true;
    }
    return;
  }

  // Gegenrichtung ab kStep ist eine echte Umkehr und folgt sofort.
  t.up = up;
  t.rest = 0;
  t.locked = t.smooth;
  changed = true;
}

void loop() {
  static Track trackers[kSliderCount];
  static uint8_t prime = 8;
  static uint32_t lastSendMs = 0;
  static bool pending = true;

  logGpioInputs();

  // Die ersten Messungen nur glätten. Sonst übernimmt ein einzelner ADC-Zacken die Lautstärke.
  if (prime > 0) {
    for (int i = 0; i < kSliderCount; i++) {
      int raw = readSlider(kPins[i]);
      if (!trackers[i].ready) {
        trackers[i].smooth = raw;
        trackers[i].ready = true;
      } else {
        trackers[i].smooth += (raw - trackers[i].smooth) / 4;
      }
    }
    prime--;
    if (prime == 0) {
      for (int i = 0; i < kSliderCount; i++) {
        trackers[i].locked = trackers[i].smooth;
      }
    }
    return;
  }

  bool changed = false;
  for (int i = 0; i < kSliderCount; i++) {
    track(trackers[i], readSlider(kPins[i]), changed);
  }

  uint32_t now = millis();
  pending = pending || changed;
  if ((pending || now - lastSendMs >= kHeartbeatMs) && sendSliders(trackers)) {
    pending = false;
    lastSendMs = now;
  }
}
