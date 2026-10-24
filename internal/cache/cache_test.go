package cache

import (
	"testing"
	"time"
)

// newLocalStore crea un Store solo-memoria con reloj controlable para las pruebas.
func newLocalStore() (*Store, func(time.Duration)) {
	now := time.Now()
	s := NewStore(nil)
	s.now = func() time.Time { return now }
	advance := func(d time.Duration) { now = now.Add(d) }
	return s, advance
}

func TestLocalIncrement(t *testing.T) {
	s, advance := newLocalStore()
	for want := int64(1); want <= 3; want++ {
		if got := s.Increment("c", time.Minute); got != want {
			t.Fatalf("Increment = %d, se esperaba %d", got, want)
		}
	}
	advance(time.Minute) // vence la ventana
	if got := s.Increment("c", time.Minute); got != 1 {
		t.Errorf("tras vencer, Increment = %d, se esperaba 1", got)
	}
}

func TestLocalSetIfAbsent(t *testing.T) {
	s, advance := newLocalStore()
	if !s.SetIfAbsent("lock", time.Minute) {
		t.Fatal("la primera vez debía fijar la llave")
	}
	if s.SetIfAbsent("lock", time.Minute) {
		t.Error("la segunda vez no debía fijarla")
	}
	advance(time.Minute)
	if !s.SetIfAbsent("lock", time.Minute) {
		t.Error("tras vencer debía poder fijarla de nuevo")
	}
}

func TestLocalPutGetTake(t *testing.T) {
	s, advance := newLocalStore()
	if _, ok := s.Get("k"); ok {
		t.Error("una llave inexistente no debía existir")
	}
	s.Put("k", "v", time.Minute)
	if v, ok := s.Get("k"); !ok || v != "v" {
		t.Errorf("Get = %q, %v", v, ok)
	}
	if v, ok := s.Take("k"); !ok || v != "v" {
		t.Errorf("Take = %q, %v", v, ok)
	}
	if _, ok := s.Get("k"); ok {
		t.Error("Take debía borrar la llave")
	}

	s.Put("exp", "v", time.Minute)
	advance(time.Minute)
	if _, ok := s.Get("exp"); ok {
		t.Error("una llave vencida no debía leerse")
	}
}

func TestPurgeExpired(t *testing.T) {
	s, advance := newLocalStore()
	s.Put("a", "1", time.Minute)
	s.Put("b", "2", 10*time.Minute)
	advance(2 * time.Minute)
	s.PurgeExpired()
	if _, ok := s.Get("a"); ok {
		t.Error("a debía purgarse")
	}
	if _, ok := s.Get("b"); !ok {
		t.Error("b no debía purgarse")
	}
}

func TestRedisAvailableNil(t *testing.T) {
	if NewStore(nil).RedisAvailable() {
		t.Error("sin cliente, RedisAvailable debía ser false")
	}
}
