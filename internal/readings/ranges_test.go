package readings

import "testing"

func f(v float64) *float64 { return &v }

func TestValidateRanges(t *testing.T) {
	if err := ValidateRanges(Measures{}); err == nil {
		t.Error("una lectura vacía debía fallar")
	}
	valid := Measures{Temperature: f(24.5), Humidity: f(60), Ph: f(6.1), Atmosphere: f(1012)}
	if err := ValidateRanges(valid); err != nil {
		t.Errorf("la lectura válida no debía fallar: %v", err)
	}
	outOfRange := []Measures{
		{Temperature: f(-21)},
		{Temperature: f(61)},
		{Humidity: f(101)},
		{Ph: f(14.1)},
		{Atmosphere: f(299)},
		{Brightness: f(200001)},
	}
	for _, m := range outOfRange {
		if err := ValidateRanges(m); err == nil {
			t.Errorf("un valor fuera de rango debía fallar: %+v", m)
		}
	}
}
