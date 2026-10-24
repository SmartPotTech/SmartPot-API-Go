package events

import "testing"

type sampleEvent struct{ ID string }
type otherEvent struct{ N int }

func TestBusDispatchesByType(t *testing.T) {
	bus := NewBus()
	var got []string
	Subscribe(bus, func(e sampleEvent) { got = append(got, "a:"+e.ID) })
	Subscribe(bus, func(e sampleEvent) { got = append(got, "b:"+e.ID) })
	Subscribe(bus, func(e otherEvent) { got = append(got, "other") })

	bus.Publish(sampleEvent{ID: "1"})
	bus.Publish(otherEvent{N: 7})

	want := []string{"a:1", "b:1", "other"}
	if len(got) != len(want) {
		t.Fatalf("got %v, se esperaba %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("orden/entrega inesperada: got %v, se esperaba %v", got, want)
			break
		}
	}
}

func TestBusNoHandlers(t *testing.T) {
	// Publicar sin suscriptores no debe fallar.
	NewBus().Publish(sampleEvent{ID: "x"})
}
