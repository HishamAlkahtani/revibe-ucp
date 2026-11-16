package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestTypedCartPreservesReplacementFields(t *testing.T) {
	const payload = `{
		"id":"cart-1", "continue_url":"https://sa.revibe.me/cart/test",
		"currency":"SAR",
		"line_items":[{"id":"line-1","item":{"id":"variant-1","title":"Phone","price":120000},"quantity":2}],
		"totals":[{"type":"total","amount":240000,"display_text":"Total"}],
		"messages":[{"type":"error","code":"address_required","content":"Add an address","content_type":"plain","severity":"recoverable"}],
		"buyer":{"email":"buyer@example.com","extension":{"consent":true}},
		"discounts":{"codes":["SAVE"]},
		"fulfillment":{"methods":[{"type":"shipping","custom":{"value":123}}]},
		"attribution":{"source":"demo"},
		"payment":{"handlers":[{"id":"merchant-handler","custom":true}]}
	}`
	var cart Cart
	if err := json.Unmarshal([]byte(payload), &cart); err != nil {
		t.Fatal(err)
	}
	if cart.ID != "cart-1" || cart.ContinueURL != "https://sa.revibe.me/cart/test" {
		t.Fatal("cart identity or continuation URL lost")
	}
	write := CartWrite{
		LineItems: []CartLineInput{{Item: ItemReference{ID: cart.LineItems[0].Item.ID}, Quantity: 3}},
		Context:   locale(), CartExtensions: cart.CartExtensions,
	}
	encoded, err := json.Marshal(write)
	if err != nil {
		t.Fatal(err)
	}
	var original, replacement map[string]json.RawMessage
	_ = json.Unmarshal([]byte(payload), &original)
	if err := json.Unmarshal(encoded, &replacement); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"buyer", "discounts", "fulfillment", "attribution", "payment"} {
		var before, after json.RawMessage
		before, _ = json.Marshal(json.RawMessage(original[key]))
		after, _ = json.Marshal(json.RawMessage(replacement[key]))
		if string(before) != string(after) {
			t.Fatalf("replacement changed %s", key)
		}
	}
	for _, key := range []string{"id", "continue_url", "totals", "messages"} {
		if _, exists := replacement[key]; exists {
			t.Fatalf("replacement included read-only cart field %s", key)
		}
	}
	var lines []map[string]json.RawMessage
	_ = json.Unmarshal(replacement["line_items"], &lines)
	var item map[string]json.RawMessage
	_ = json.Unmarshal(lines[0]["item"], &item)
	if len(item) != 1 || string(item["id"]) != `"variant-1"` {
		t.Fatal("replacement must send item references only")
	}
	view := cartView(&cart)
	if view.LineItems[0].Item.Title != "Phone" || view.Totals[0].Amount != 240000 || view.Messages[0].Code != "address_required" {
		t.Fatal("frontend cart details lost")
	}
}

func TestTypedToolOutputShapes(t *testing.T) {
	products := []Product{{Title: "Phone", Variants: []Variant{}}}
	encoded, err := toolOutput(products, nil)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []Product
	if err := json.Unmarshal(encoded, &decoded); err != nil || !reflect.DeepEqual(products, decoded) {
		t.Fatal("search output must remain a product array")
	}
	encoded, err = toolOutput((*CartView)(nil), nil)
	if err != nil || string(encoded) != "null" {
		t.Fatal("empty cart must remain JSON null")
	}
	encoded, err = toolOutput(&CartView{CheckoutURL: "https://sa.revibe.me/cart/test"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var view CartView
	if err := json.Unmarshal(encoded, &view); err != nil || view.CheckoutURL == "" {
		t.Fatal("checkout tool output lost its continuation URL")
	}
}
