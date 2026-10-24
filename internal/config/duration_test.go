package config

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
	}{
		{"P7D", 7 * 24 * time.Hour},
		{"PT5M", 5 * time.Minute},
		{"PT5S", 5 * time.Second},
		{"PT1M", time.Minute},
		{"PT2M", 2 * time.Minute},
		{"PT30M", 30 * time.Minute},
		{"PT10M", 10 * time.Minute},
		{"PT10S", 10 * time.Second},
		{"PT24H", 24 * time.Hour},
		{"P1DT2H3M4S", 24*time.Hour + 2*time.Hour + 3*time.Minute + 4*time.Second},
		{"PT1.5S", 1500 * time.Millisecond},
		{"-PT5M", -5 * time.Minute},
		{"pt5m", 5 * time.Minute},
		// Formato nativo de Go admitido como comodidad.
		{"90s", 90 * time.Second},
		{"2h30m", 2*time.Hour + 30*time.Minute},
	}
	for _, c := range cases {
		got, err := ParseDuration(c.in)
		if err != nil {
			t.Fatalf("ParseDuration(%q) error inesperado: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("ParseDuration(%q) = %v, se esperaba %v", c.in, got, c.want)
		}
	}
}

func TestParseDurationInvalid(t *testing.T) {
	for _, in := range []string{"", "P", "PT", "P5", "PTX", "P1Y", "P1M", "abc"} {
		if _, err := ParseDuration(in); err == nil {
			t.Errorf("ParseDuration(%q) debía fallar", in)
		}
	}
}
