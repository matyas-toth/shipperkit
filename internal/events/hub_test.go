package events

import "testing"

func TestSlowSubscribersAreDisconnectedWithoutBlocking(t *testing.T) {
	h := New()
	ch, unsubscribe, ok := h.Subscribe()
	if !ok {
		t.Fatal("subscribe failed")
	}
	for i := 0; i < 20; i++ {
		h.Publish()
	}
	for range ch {
	}
	unsubscribe()
	unsubscribe()
	_, cancel, ok := h.Subscribe()
	if !ok {
		t.Fatal("slow client leaked its slot")
	}
	cancel()
}
func TestSubscriberLimit(t *testing.T) {
	h := New()
	for i := 0; i < 64; i++ {
		_, _, ok := h.Subscribe()
		if !ok {
			t.Fatal(i)
		}
	}
	if _, _, ok := h.Subscribe(); ok {
		t.Fatal("subscriber cap was not enforced")
	}
}
