package main

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
)

func (c *Commerce) search(ctx context.Context, s *Session, input SearchInput) (any, error) {
	if strings.TrimSpace(input.Query) == "" || len(input.Query) > 300 {
		return nil, errors.New("enter a product name of up to 300 characters")
	}
	max := math.Inf(1)
	filters := object{"available": true}
	if input.MaxPrice != nil {
		if *input.MaxPrice <= 0 || *input.MaxPrice > 1000000 {
			return nil, errors.New("invalid SAR budget")
		}
		max = math.Round(*input.MaxPrice * 100)
		filters["price"] = object{"max": max}
	}
	var data struct{ Products []Product }
	err := c.call(ctx, "search_catalog", object{"catalog": object{"query": input.Query, "context": locale(), "pagination": object{"limit": 4}, "filters": filters}}, &data)
	if err != nil {
		return nil, err
	}
	products := []Product{}
	for _, p := range data.Products {
		variants := []Variant{}
		for _, v := range p.Variants {
			if !v.Availability.Available || v.Price.Currency != "SAR" || float64(v.Price.Amount) > max {
				continue
			}
			for _, media := range v.Media {
				if media.Type == "image" {
					v.Image = media.URL
					break
				}
			}
			v.Media = nil
			variants = append(variants, v)
		}
		if len(variants) > 0 {
			products = append(products, Product{p.Title, variants})
		}
	}
	s.Products = products
	return products, nil
}

func (c *Commerce) add(ctx context.Context, s *Session, input AddInput) (any, error) {
	known := false
	for _, p := range s.Products {
		for _, v := range p.Variants {
			if v.ID == input.VariantID {
				known = true
			}
		}
	}
	if !known {
		return nil, errors.New("search for an available variant first")
	}
	if input.Quantity < 1 || input.Quantity > 10 {
		return nil, errors.New("quantity must be 1–10")
	}
	current := Cart{}
	if s.Cart != nil {
		if err := c.call(ctx, "get_cart", object{"id": s.Cart["id"]}, &current); err != nil {
			return nil, err
		}
	}
	// Cart updates replace the resource: send every line and writable field.
	lines := []object{}
	found := false
	var items []struct {
		Item struct {
			ID string `json:"id"`
		} `json:"item"`
		Quantity int `json:"quantity"`
	}
	if current["line_items"] != nil {
		data, err := json.Marshal(current["line_items"])
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &items); err != nil {
			return nil, errors.New("invalid cart lines from Revibe")
		}
	}
	for _, line := range items {
		quantity := line.Quantity
		if line.Item.ID == input.VariantID {
			quantity += input.Quantity
			found = true
		}
		if quantity > 10 {
			return nil, errors.New("maximum 10 units per variant")
		}
		if line.Item.ID == "" || quantity < 1 {
			return nil, errors.New("invalid cart line from Revibe")
		}
		lines = append(lines, object{"item": object{"id": line.Item.ID}, "quantity": quantity})
	}
	if !found {
		lines = append(lines, object{"item": object{"id": input.VariantID}, "quantity": input.Quantity})
	}
	if len(lines) > 20 {
		return nil, errors.New("maximum 20 cart lines")
	}
	cart := object{"line_items": lines, "context": locale()}
	for _, key := range []string{"buyer", "discounts", "fulfillment", "attribution", "payment"} {
		if current[key] != nil {
			cart[key] = current[key]
		}
	}
	name := "create_cart"
	body := object{"cart": cart}
	if s.Cart != nil {
		name = "update_cart"
		body["id"] = current["id"]
	}
	var updated Cart
	if err := c.call(ctx, name, body, &updated); err != nil {
		return nil, err
	}
	s.Cart, s.CheckoutURL = updated, ""
	if value, ok := updated["continue_url"].(string); ok {
		if link, err := checkoutURL(value); err == nil {
			s.CheckoutURL = link
		}
	}
	return cartView(s.Cart), nil
}

func (c *Commerce) refresh(ctx context.Context, s *Session) (any, error) {
	if s.Cart == nil {
		return nil, nil
	}
	var cart Cart
	if err := c.call(ctx, "get_cart", object{"id": s.Cart["id"]}, &cart); err != nil {
		return nil, err
	}
	s.Cart, s.CheckoutURL = cart, ""
	if value, ok := cart["continue_url"].(string); ok {
		if link, err := checkoutURL(value); err == nil {
			s.CheckoutURL = link
		}
	}
	return cartView(cart), nil
}

func (c *Commerce) checkout(ctx context.Context, s *Session) (any, error) {
	if s.Cart == nil {
		return nil, errors.New("add a device first")
	}
	var cart Cart
	if err := c.call(ctx, "get_cart", object{"id": s.Cart["id"]}, &cart); err != nil {
		return nil, err
	}
	s.Cart = cart
	value, _ := cart["continue_url"].(string)
	link, err := checkoutURL(value)
	if err != nil {
		return nil, err
	}
	s.CheckoutURL = link
	requestLog(ctx).Info("checkout handoff ready")
	result := cartView(cart)
	result["checkout_url"] = link
	return result, nil
}

func locale() object { return object{"address_country": "SA", "currency": "SAR"} }
func cartView(cart Cart) object {
	if cart == nil {
		return nil
	}
	return object{"line_items": cart["line_items"], "totals": cart["totals"], "currency": cart["currency"], "messages": cart["messages"]}
}
