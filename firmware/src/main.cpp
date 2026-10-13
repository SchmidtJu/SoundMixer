#include <Arduino.h>

// Slider 1..4, links nach rechts. Nur ADC1, keine Strap-/USB-/LED-Pins.
// Poti: Außenpins an 3V3 und GND, Schleifer an den GPIO.
static const int kPins[4] = {1, 2, 4, 5};
static const int kSliderCount = 4;
static const int kSamples = 8;
static const int kMinIntervalMs = 30;
static const int kHeartbeatMs = 1000;
static const int kChangeThreshold = 4;

static int readSlider(int pin) {
  uint32_t sum = 0;
  for (int i = 0; i < kSamples; i++) {
    sum += analogRead(pin);
  }
  int scaled = (int)((sum / kSamples) >> 2);  // 12 Bit (0..4095) -> 0..1023
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
  for (int i = 0; i < kSliderCount; i++) {
    pinMode(kPins[i], INPUT);
    analogSetPinAttenuation(kPins[i], ADC_11db);
  }

  Serial.begin(115200);
  // Nicht blockieren, wenn der PC den COM-Port noch nicht geöffnet hat.
  Serial.setTxTimeoutMs(0);
  delay(200);
}

void loop() {
  static int lastSent[kSliderCount] = {-999, -999, -999, -999};
  static uint32_t lastSendMs = 0;

  int values[kSliderCount];
  bool changed = false;
  for (int i = 0; i < kSliderCount; i++) {
    values[i] = readSlider(kPins[i]);
    int delta = values[i] - lastSent[i];
    if (delta < 0) {
      delta = -delta;
    }
    if (delta >= kChangeThreshold) {
      changed = true;
    }
  }

  uint32_t now = millis();
  uint32_t elapsed = now - lastSendMs;
  if ((changed && elapsed >= kMinIntervalMs) || elapsed >= kHeartbeatMs) {
    Serial.printf("%d|%d|%d|%d\n", values[0], values[1], values[2], values[3]);
    for (int i = 0; i < kSliderCount; i++) {
      lastSent[i] = values[i];
    }
    lastSendMs = now;
  }

  delay(5);
}
