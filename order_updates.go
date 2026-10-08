package main

import (
	"errors"
	"strings"
)

type orderUpdate struct {
	ID       string `json:"id"`
	OrderID  string `json:"order_id"`
	Stage    string `json:"stage"`
	MemberID string `json:"member_id"`
}

type announcement struct {
	ID      string `json:"id"`
	Message string `json:"message"`
}

func (u orderUpdate) event() (string, error) {
	if u.ID == "" || u.OrderID == "" || u.MemberID == "" {
		return "", errors.New("id, order_id and member_id are required")
	}
	switch u.Stage {
	case "checkout":
		return "order.checkout", nil
	case "fulfillment":
		return "order.fulfillment", nil
	case "receipt":
		return "order.receipt", nil
	case "update":
		return "order.update", nil
	default:
		return "", errors.New("stage must be checkout, fulfillment, receipt or update")
	}
}

func (a announcement) validate() error {
	if strings.TrimSpace(a.ID) == "" || strings.TrimSpace(a.Message) == "" {
		return errors.New("id and message are required")
	}
	return nil
}
