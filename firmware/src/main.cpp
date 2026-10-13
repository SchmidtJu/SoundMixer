#include <Arduino.h>

// Slider 1..4, links nach rechts. Nur ADC1, keine Strap-/USB-/LED-Pins.
// Poti: Außenpins an 3V3 und GND, Schleifer an den GPIO.
static const int kPins[4] = {1, 2, 4, 5};
static const int kSliderCount = 4;
// Diagnose: GPIO 1..4 sind die ersten vier ADC1-Kanäle, Rohwerte 0..4095.
static const int kGpioLogMs = 500;
static const int kSamples = 9;
static const int kHeartbeatMs = 1000;
// Glättung muss erst klar und mehrere Messungen lang in dieselbe Richtung laufen.
static const int kIdleBand = 36;
// Danach folgt die Lautstärke in kleineren Stufen, ohne auf jeden ADC-Zacken zu hören.
static const int kTrackBand = 18;
static const int kSettle = 4;

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

static void logGpioInputs() {
  static uint32_t lastMs = 0;
  uint32_t now = millis();
  if (lastMs != 0 && now - lastMs < (uint32_t)kGpioLogMs) {
    return;
  }
  lastMs = now;
  Serial.printf("gpio 1=%d 2=%d 3=%d 4=%d\n", readAdcRaw(1), readAdcRaw(2), readAdcRaw(3),
                readAdcRaw(4));
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
  analogSetPinAttenuation(3, ADC_11db);

  Serial.begin(115200);
  // Nicht blockieren, wenn der PC den COM-Port noch nicht geöffnet hat.
  Serial.setTxTimeoutMs(0);
  delay(200);
  Serial.println("gpio pin1-4=adc");
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

  if (dist < kTrackBand) {
    t.hold = 0;
    if (t.rest < 255) {
      t.rest++;
    }
    if (t.rest < kSettle) {
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

  // Gegenrichtung unter der Stillstandsschwelle ist Reststreuung, kein neuer Zug.
  if (up != t.up && dist < kIdleBand) {
    if (t.rest < 255) {
      t.rest++;
    }
    if (t.rest >= kSettle) {
      t.moving = false;
      t.rest = 0;
    }
    return;
  }

  t.up = up;
  t.rest = 0;
  t.locked = t.smooth;
  changed = true;
}

void loop() {
  static Track trackers[kSliderCount];
  static uint8_t prime = 8;
  static uint32_t lastSendMs = 0;

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
      Serial.printf("%d|%d|%d|%d\n", trackers[0].locked, trackers[1].locked, trackers[2].locked,
                    trackers[3].locked);
      lastSendMs = millis();
    }
    return;
  }

  bool changed = false;
  for (int i = 0; i < kSliderCount; i++) {
    track(trackers[i], readSlider(kPins[i]), changed);
  }

  uint32_t now = millis();
  if (changed || now - lastSendMs >= kHeartbeatMs) {
    Serial.printf("%d|%d|%d|%d\n", trackers[0].locked, trackers[1].locked, trackers[2].locked,
                  trackers[3].locked);
    lastSendMs = now;
  }
}
