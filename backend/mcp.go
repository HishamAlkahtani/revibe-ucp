package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCP handles the protocol. Commerce handles UCP's product and cart rules.
type Commerce struct {
	api      *API
	profile  string
	mu       sync.Mutex
	endpoint string
}

func merchantURL(value string) (string, error) {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.User != nil || (u.Host != "sa.revibe.me" && u.Host != "revibe-sa.myshopify.com") {
		return "", errors.New("unexpected merchant URL")
	}
	return u.String(), nil
}

// A merchant can hand off to Shop; MCP discovery still only trusts Revibe hosts.
func checkoutURL(value string) (string, error) {
	u, err := url.Parse(value)
	if err == nil && u.Scheme == "https" && u.Host == "shop.app" && u.User == nil {
		return u.String(), nil
	}
	return merchantURL(value)
}

func (c *Commerce) discover(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.endpoint != "" {
		return c.endpoint, nil
	}
	var profile struct {
		UCP struct {
			Services map[string][]struct{ Transport, Version, Endpoint string }
		}
	}
	if err := c.api.request(ctx, "https://sa.revibe.me/.well-known/ucp", nil, "", &profile); err != nil {
		return "", err
	}
	for _, service := range profile.UCP.Services["dev.ucp.shopping"] {
		if service.Transport == "mcp" && service.Version == "2026-08-25" {
			endpoint, err := merchantURL(service.Endpoint)
			if err != nil {
				return "", err
			}
			c.endpoint = endpoint
			requestLog(ctx).Info("Revibe UCP discovered", "endpoint", endpoint, "version", service.Version)
			return endpoint, nil
		}
	}
	return "", errors.New("unsupported Revibe UCP version")
}

func (c *Commerce) call(ctx context.Context, name string, args MerchantArguments, out any) (err error) {
	started := time.Now()
	defer func() {
		requestLog(ctx).Info("Revibe call finished", "tool", name, "ok", err == nil, "duration", time.Since(started).Round(time.Millisecond))
	}()
	endpoint, err := c.discover(ctx)
	if err != nil {
		return err
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "revibe-chat", Version: "1.0.0"}, nil)
	transport := &mcp.StreamableClientTransport{Endpoint: endpoint, HTTPClient: c.api.client, DisableStandaloneSSE: true, MaxRetries: -1}
	// A short-lived connection avoids sharing MCP lifecycle state between shoppers.
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("connect to Revibe MCP: %w", err)
	}
	defer session.Close()
	args.Meta = &MerchantMeta{Agent: MerchantAgent{Profile: c.profile}}
	traceTool(ctx, "MCP", "request", object{"method": "tools/call", "params": object{"name": name, "arguments": args}})
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		traceTool(ctx, "MCP", "error", object{"name": name, "error": err.Error()})
		return fmt.Errorf("Revibe %s: %w", name, err)
	}
	traceTool(ctx, "MCP", "response", object{"name": name, "result": result})
	return decodeToolResult(result, out)
}

func decodeToolResult(result *mcp.CallToolResult, out any) error {
	if result.IsError {
		return errors.New("Revibe could not complete that action")
	}
	var data []byte
	if result.StructuredContent != nil {
		var err error
		data, err = json.Marshal(result.StructuredContent)
		if err != nil {
			return err
		}
	} else {
		for _, content := range result.Content {
			if text, ok := content.(*mcp.TextContent); ok {
				data = []byte(text.Text)
				break
			}
		}
	}
	var status struct{ UCP struct{ Status string } }
	if err := json.Unmarshal(data, &status); err != nil {
		return fmt.Errorf("invalid UCP result: %w", err)
	}
	if status.UCP.Status == "error" {
		return errors.New("Revibe could not complete that action")
	}
	return json.Unmarshal(data, out)
}
