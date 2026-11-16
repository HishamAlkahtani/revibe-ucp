package main

import (
	"context"
	"encoding/json"
	"log"
	"net/url"
	"os"
	"strings"
)

// Keep payload shape and public variant IDs, but redact buyer/payment credentials.
func traceTool(_ context.Context, layer, direction string, payload any) {
	if os.Getenv("LOG_TOOL_PAYLOADS") == "false" {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("%s %s: cannot encode payload", layer, direction)
		return
	}
	var value any
	if json.Unmarshal(data, &value) != nil {
		return
	}
	pretty, err := json.MarshalIndent(redactPayload("", value), "", "  ")
	if err == nil {
		log.Printf("%s %s\n%s", layer, direction, pretty)
	}
}

func redactPayload(key string, value any) any {
	normalized := strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(key))
	switch normalized {
	case "authorization", "apikey", "accesstoken", "refreshtoken", "token", "shoptoken", "authjwt", "secret", "clientsecret", "password", "credential", "credentials", "payment", "buyer", "buyerip", "shopifybuyerip", "email", "phonenumber", "phone", "firstname", "lastname", "streetaddress", "extendedaddress", "addresslocality", "addressregion", "postalcode", "address", "destinations":
		if value != nil {
			return "[redacted]"
		}
	}
	switch v := value.(type) {
	case map[string]any:
		result := map[string]any{}
		for k, child := range v {
			result[k] = redactPayload(k, child)
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, child := range v {
			result[i] = redactPayload(key, child)
		}
		return result
	case string:
		if normalized == "id" && !strings.HasPrefix(v, "gid://shopify/ProductVariant/") && !strings.HasPrefix(v, "gid://shopify/Product/") {
			return "[redacted]"
		}
		// MCP can duplicate structuredContent as a JSON string in content[].text.
		var embedded any
		if json.Unmarshal([]byte(v), &embedded) == nil {
			switch embedded.(type) {
			case map[string]any, []any:
				data, _ := json.Marshal(redactPayload(key, embedded))
				return string(data)
			}
		}
		for _, resource := range []string{"Cart", "Checkout", "CartLine", "Order"} {
			prefix := "gid://shopify/" + resource + "/"
			if strings.HasPrefix(v, prefix) {
				return prefix + "[redacted]"
			}
		}
		u, err := url.Parse(v)
		if err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Host != "" {
			u.User, u.Fragment = nil, ""
			query := u.Query()
			for k := range query {
				query.Set(k, "[redacted]")
			}
			u.RawQuery = query.Encode()
			if normalized == "continueurl" || normalized == "checkouturl" {
				u.Path, u.RawPath = "/[redacted]", ""
			}
			return u.String()
		}
		return v
	default:
		return value
	}
}
