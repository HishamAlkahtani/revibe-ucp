package main

import (
	"context"
	"errors"
	"math"
	"strings"
)

func (c *Commerce) search(ctx context.Context, s *Session, input SearchInput) ([]Product, error) {
	if strings.TrimSpace(input.Query) == "" || len(input.Query) > 300 {
		return nil, errors.New("enter a product name of up to 300 characters")
	}
	max := math.Inf(1)
	filters := CatalogFilters{Available: true}
	if input.MaxPrice != nil {
		if *input.MaxPrice <= 0 || *input.MaxPrice > 1000000 {
			return nil, errors.New("invalid SAR budget")
		}
		max = math.Round(*input.MaxPrice * 100)
		filters.Price = &CatalogPriceFilter{Max: max}
	}
	var data CatalogResult
	err := c.call(ctx, "search_catalog", MerchantArguments{Catalog: &CatalogSearch{Query: input.Query, Context: locale(), Pagination: CatalogPagination{Limit: 4}, Filters: filters}}, &data)
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

func (c *Commerce) add(ctx context.Context, s *Session, input AddInput) (*CartView, error) {
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
		if err := c.call(ctx, "get_cart", MerchantArguments{ID: s.Cart.ID}, &current); err != nil {
			return nil, err
		}
	}
	// Cart updates replace the resource: send every line and writable field.
	lines := []CartLineInput{}
	found := false
	for _, line := range current.LineItems {
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
		lines = append(lines, CartLineInput{Item: ItemReference{ID: line.Item.ID}, Quantity: quantity})
	}
	if !found {
		lines = append(lines, CartLineInput{Item: ItemReference{ID: input.VariantID}, Quantity: input.Quantity})
	}
	if len(lines) > 20 {
		return nil, errors.New("maximum 20 cart lines")
	}
	cart := CartWrite{LineItems: lines, Context: locale(), CartExtensions: current.CartExtensions}
	name := "create_cart"
	body := MerchantArguments{Cart: &cart}
	if s.Cart != nil {
		name = "update_cart"
		body.ID = current.ID
	}
	var updated Cart
	if err := c.call(ctx, name, body, &updated); err != nil {
		return nil, err
	}
	s.Cart, s.CheckoutURL = &updated, ""
	if link, err := checkoutURL(updated.ContinueURL); err == nil {
		s.CheckoutURL = link
	}
	return cartView(s.Cart), nil
}

func (c *Commerce) refresh(ctx context.Context, s *Session) (*CartView, error) {
	if s.Cart == nil {
		return nil, nil
	}
	var cart Cart
	if err := c.call(ctx, "get_cart", MerchantArguments{ID: s.Cart.ID}, &cart); err != nil {
		return nil, err
	}
	s.Cart, s.CheckoutURL = &cart, ""
	if link, err := checkoutURL(cart.ContinueURL); err == nil {
		s.CheckoutURL = link
	}
	return cartView(&cart), nil
}

func (c *Commerce) checkout(ctx context.Context, s *Session) (*CartView, error) {
	if s.Cart == nil {
		return nil, errors.New("add a device first")
	}
	var cart Cart
	if err := c.call(ctx, "get_cart", MerchantArguments{ID: s.Cart.ID}, &cart); err != nil {
		return nil, err
	}
	s.Cart = &cart
	link, err := checkoutURL(cart.ContinueURL)
	if err != nil {
		return nil, err
	}
	s.CheckoutURL = link
	requestLog(ctx).Info("checkout handoff ready")
	result := cartView(&cart)
	result.CheckoutURL = link
	return result, nil
}

func locale() ShoppingContext { return ShoppingContext{AddressCountry: "SA", Currency: "SAR"} }
func cartView(cart *Cart) *CartView {
	if cart == nil {
		return nil
	}
	return &CartView{LineItems: cart.LineItems, Totals: cart.Totals, Currency: cart.Currency, Messages: cart.Messages}
}
