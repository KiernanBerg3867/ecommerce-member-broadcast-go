package main

import "testing"

func TestOrderEvent(t *testing.T) {
	tests := []struct {
		stage, want string
		valid       bool
	}{
		{"checkout", "order.checkout", true},
		{"fulfillment", "order.fulfillment", true},
		{"receipt", "order.receipt", true},
		{"update", "order.update", true},
		{"cancelled", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.stage, func(t *testing.T) {
			got, err := (orderUpdate{ID: "evt-1", OrderID: "ord-1", MemberID: "member-1", Stage: tt.stage}).event()
			if (err == nil) != tt.valid || got != tt.want {
				t.Fatalf("event = %q, err = %v; want %q, valid %v", got, err, tt.want, tt.valid)
			}
		})
	}
}
