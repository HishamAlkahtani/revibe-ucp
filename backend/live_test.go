package main

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"
)

// Opt-in: creates a real merchant cart, but never places an order or pays.
func TestLiveAnonymousCart(t *testing.T) {
	if os.Getenv("REVIBE_LIVE_TEST") != "1" {
		t.Skip("set REVIBE_LIVE_TEST=1 to test the live anonymous cart flow")
	}
	t.Setenv("LOG_TOOL_PAYLOADS", "false")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	c := &Commerce{api: &API{client: &http.Client{Timeout: 30 * time.Second}}, profile: "https://shopify.dev/ucp/agent-profiles/examples/2026-08-25/valid-with-capabilities.json"}
	s := &Session{}
	if _, err := c.search(ctx, s, SearchInput{Query: "iPhone 13"}); err != nil {
		t.Fatal(err)
	}
	if len(s.Products) == 0 || len(s.Products[0].Variants) == 0 {
		t.Fatal("no available variants returned")
	}
	t.Log("anonymous catalog search succeeded")
	id := s.Products[0].Variants[0].ID
	if _, err := c.add(ctx, s, AddInput{VariantID: id, Quantity: 1}); err != nil {
		t.Fatal(err)
	}
	t.Log("anonymous create_cart succeeded")
	if _, err := c.add(ctx, s, AddInput{VariantID: id, Quantity: 1}); err != nil {
		t.Fatal(err)
	}
	t.Log("anonymous get_cart and update_cart succeeded")
	if _, err := c.checkout(ctx, s); err != nil {
		t.Fatal(err)
	}
	if s.CheckoutURL == "" {
		t.Fatal("no validated continuation URL")
	}
	t.Log("anonymous cart refresh and validated browser handoff succeeded")
}
