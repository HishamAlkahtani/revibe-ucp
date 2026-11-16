package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"
	"testing"
)

func TestPayloadLogsAreReadableAndRedacted(t *testing.T) {
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)
	t.Setenv("LOG_TOOL_PAYLOADS", "true")
	payload := object{
		"method": "tools/call",
		"params": object{"name": "create_cart", "arguments": object{
			"cart": object{"line_items": []object{{"item": object{"id": "gid://shopify/ProductVariant/123"}, "quantity": 1}}, "buyer": object{"email": "private@example.com"}, "payment": object{"token": "secret-token"}},
		}},
		"continue_url": "https://shop.app/checkout/private-checkout?key=private-key",
		"id":           "gid://shopify/Checkout/private-id",
		"content":      []object{{"type": "text", "text": `{"buyer":{"email":"nested@example.com"},"access_token":"nested-secret"}`}},
	}
	traceTool(context.Background(), "MCP", "request", payload)
	text := output.String()
	for _, secret := range []string{"private@example.com", "secret-token", "private-checkout", "private-key", "private-id", "nested@example.com", "nested-secret"} {
		if strings.Contains(text, secret) {
			t.Fatalf("leaked sensitive value %q", secret)
		}
	}
	for _, useful := range []string{"MCP request", "tools/call", "create_cart", "line_items", "gid://shopify/ProductVariant/123", "shop.app", "\n  \""} {
		if !strings.Contains(text, useful) {
			t.Fatalf("missing useful structure: %q", useful)
		}
	}
	encoded, _ := json.Marshal(payload)
	if !strings.Contains(string(encoded), "private@example.com") {
		t.Fatal("logging mutated the actual request")
	}
	output.Reset()
	t.Setenv("LOG_TOOL_PAYLOADS", "false")
	traceTool(context.Background(), "MCP", "request", payload)
	if output.Len() != 0 {
		t.Fatal("trace did not disable")
	}
}

func TestCheckoutHandoffHosts(t *testing.T) {
	for _, value := range []string{"https://sa.revibe.me/checkout/example", "https://revibe-sa.myshopify.com/checkout/example", "https://shop.app/checkout/example"} {
		if _, err := checkoutURL(value); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []string{"https://evil.example/pay", "http://shop.app/pay", "https://shop.app.evil.example/pay", "https://user:pass@shop.app/pay"} {
		if _, err := checkoutURL(value); err == nil {
			t.Fatalf("accepted %s", value)
		}
	}
	if _, err := merchantURL("https://shop.app/mcp"); err == nil {
		t.Fatal("Shop must not become a merchant MCP endpoint")
	}
}
