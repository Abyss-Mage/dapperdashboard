package postgres

import (
	"context"
	"testing"
	"time"
)

// A booking made by the team sends the confirmation like a website booking; one waiting for approval waits until it is approved.
func TestAnnounceBooking(t *testing.T) {
	s := &Store{}
	got := make(chan BookingEmailEvent, 2)
	s.SetBookingEventHook(func(e BookingEmailEvent) { got <- e })
	s.AnnounceBooking(context.Background(), Booking{ID: "pending-one", Status: "pending"})
	s.AnnounceBooking(context.Background(), Booking{ID: "approved-one", Status: "confirmed"})
	select {
	case e := <-got:
		if e.Kind != "booking_confirmation" || e.BookingID != "approved-one" {
			t.Fatalf("wrong announcement: %+v", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("an approved team booking was not announced")
	}
	select {
	case e := <-got:
		t.Fatalf("a pending booking was announced: %+v", e)
	case <-time.After(200 * time.Millisecond):
	}
	// Quiet work (internal clean-ups) never announces.
	s.AnnounceBooking(QuietBookingEvents(context.Background()), Booking{ID: "quiet", Status: "confirmed"})
	select {
	case e := <-got:
		t.Fatalf("quiet work was announced: %+v", e)
	case <-time.After(200 * time.Millisecond):
	}
}
