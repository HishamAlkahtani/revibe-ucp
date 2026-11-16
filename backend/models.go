package main

import "encoding/json"

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

type Cart struct {
	ID          string        `json:"id"`
	ContinueURL string        `json:"continue_url"`
	Currency    string        `json:"currency"`
	LineItems   []CartLine    `json:"line_items"`
	Totals      []CartTotal   `json:"totals"`
	Messages    []CartMessage `json:"messages"`
	// Preserve merchant extension payloads unchanged in replacement updates.
	CartExtensions
}

type CartExtensions struct {
	Buyer       json.RawMessage `json:"buyer,omitempty"`
	Discounts   json.RawMessage `json:"discounts,omitempty"`
	Fulfillment json.RawMessage `json:"fulfillment,omitempty"`
	Attribution json.RawMessage `json:"attribution,omitempty"`
	Payment     json.RawMessage `json:"payment,omitempty"`
}

type CartItem struct {
	ID       string `json:"id"`
	Title    string `json:"title,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	Price    int    `json:"price,omitempty"`
}

type CartLine struct {
	ID       string      `json:"id,omitempty"`
	Item     CartItem    `json:"item"`
	Quantity int         `json:"quantity"`
	Totals   []CartTotal `json:"totals,omitempty"`
}

type CartTotal struct {
	Type        string `json:"type"`
	Amount      int    `json:"amount"`
	DisplayText string `json:"display_text"`
}

type CartMessage struct {
	Code        string `json:"code"`
	Content     string `json:"content"`
	ContentType string `json:"content_type"`
	Severity    string `json:"severity"`
	Type        string `json:"type"`
}

type CartView struct {
	LineItems   []CartLine    `json:"line_items"`
	Totals      []CartTotal   `json:"totals"`
	Currency    string        `json:"currency"`
	Messages    []CartMessage `json:"messages"`
	CheckoutURL string        `json:"checkout_url,omitempty"`
}

type ShoppingContext struct {
	AddressCountry string `json:"address_country"`
	Currency       string `json:"currency"`
}

type CatalogSearch struct {
	Query      string            `json:"query"`
	Context    ShoppingContext   `json:"context"`
	Pagination CatalogPagination `json:"pagination"`
	Filters    CatalogFilters    `json:"filters"`
}
type CatalogPagination struct {
	Limit int `json:"limit"`
}
type CatalogFilters struct {
	Available bool                `json:"available"`
	Price     *CatalogPriceFilter `json:"price,omitempty"`
}
type CatalogPriceFilter struct {
	Max float64 `json:"max"`
}
type CatalogResult struct {
	Products []Product `json:"products"`
}

// Writes use item references, never the merchant's read-only item details.
type CartLineInput struct {
	Item     ItemReference `json:"item"`
	Quantity int           `json:"quantity"`
}
type ItemReference struct {
	ID string `json:"id"`
}
type CartWrite struct {
	LineItems []CartLineInput `json:"line_items"`
	Context   ShoppingContext `json:"context"`
	CartExtensions
}
type MerchantArguments struct {
	ID      string         `json:"id,omitempty"`
	Catalog *CatalogSearch `json:"catalog,omitempty"`
	Cart    *CartWrite     `json:"cart,omitempty"`
	Meta    *MerchantMeta  `json:"meta,omitempty"`
}
type MerchantMeta struct {
	Agent MerchantAgent `json:"ucp-agent"`
}
type MerchantAgent struct {
	Profile string `json:"profile"`
}
type ToolError struct {
	Error string `json:"error"`
}

type SearchInput struct {
	Query    string   `json:"query"`
	MaxPrice *float64 `json:"max_price_sar"`
}
type AddInput struct {
	VariantID string `json:"variant_id"`
	Quantity  int    `json:"quantity"`
}

// Model-facing tool schemas are distinct from merchant MCP arguments.
type ToolDefinition struct {
	Type        string         `json:"type"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Strict      bool           `json:"strict"`
	Parameters  ToolParameters `json:"parameters"`
}
type ToolParameters struct {
	Type                 string                  `json:"type"`
	Properties           map[string]ToolProperty `json:"properties"`
	Required             []string                `json:"required"`
	AdditionalProperties bool                    `json:"additionalProperties"`
}
type ToolProperty struct {
	Type json.RawMessage `json:"type"`
}
type FunctionCall struct {
	Name      string `json:"name"`
	CallID    string `json:"call_id"`
	Arguments string `json:"arguments"`
}
type ModelContent struct {
	Type    string `json:"type"`
	Text    string `json:"text"`
	Refusal string `json:"refusal"`
}
type ModelOutputItem struct {
	Type string `json:"type"`
	FunctionCall
	Content []ModelContent `json:"content"`
}
type ModelRequest struct {
	Model             string            `json:"model"`
	Instructions      string            `json:"instructions"`
	Input             []json.RawMessage `json:"input"`
	Tools             []ToolDefinition  `json:"tools"`
	ParallelToolCalls bool              `json:"parallel_tool_calls"`
	Store             bool              `json:"store"`
	Stream            bool              `json:"stream"`
	MaxOutputTokens   int               `json:"max_output_tokens"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type FunctionCallOutput struct {
	Type   string `json:"type"`
	CallID string `json:"call_id"`
	Output string `json:"output"`
}
