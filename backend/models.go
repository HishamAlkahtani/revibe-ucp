package main

type object = map[string]any

type Price struct {
	Amount   int    `json:"amount"`
	Currency string `json:"currency"`
}

type Variant struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Price        Price  `json:"price"`
	Image        string `json:"image,omitempty"`
	Availability struct {
		Available bool `json:"available"`
	} `json:"availability"`
	Media []struct {
		Type string `json:"type"`
		URL  string `json:"url"`
	} `json:"media,omitempty"`
}

type Product struct {
	Title    string    `json:"title"`
	Variants []Variant `json:"variants"`
}

// Keep the full merchant cart so replacement updates preserve writable fields.
type Cart map[string]any

type SearchInput struct {
	Query    string   `json:"query"`
	MaxPrice *float64 `json:"max_price_sar"`
}
type AddInput struct {
	VariantID string `json:"variant_id"`
	Quantity  int    `json:"quantity"`
}
