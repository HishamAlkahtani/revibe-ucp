package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCommerceFlow(t *testing.T) {
	var writes []object
	var toolNames []string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("Shopify-Buyer-IP") != "" {
			t.Error("unexpected merchant authentication headers")
		}
		var request struct {
			JSONRPC string `json:"jsonrpc"`
			ID      any    `json:"id"`
			Method  string `json:"method"`
			Params  struct {
				Name      string
				Arguments object
			}
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if request.Method == "initialize" {
			_ = json.NewEncoder(w).Encode(object{"jsonrpc": "2.0", "id": request.ID, "result": object{"protocolVersion": "2025-06-18", "capabilities": object{"tools": object{}}, "serverInfo": object{"name": "test", "version": "1.0"}}})
			return
		}
		if request.Method == "notifications/initialized" {
			w.WriteHeader(202)
			return
		}
		if request.Method == "tools/call" {
			toolNames = append(toolNames, request.Params.Name)
		}
		var result any = object{"id": "cart-1", "currency": "SAR", "continue_url": "https://sa.revibe.me/cart/test", "line_items": []object{{"item": object{"id": "variant-1", "title": "Phone"}, "quantity": 1}}, "buyer": object{"email": "buyer@example.com"}, "discounts": object{"codes": []string{"SAVE"}}}
		switch request.Params.Name {
		case "search_catalog":
			result = object{"products": []object{{"title": "Phone", "variants": []object{
				{"id": "variant-1", "title": "128 GB", "price": object{"amount": 120000, "currency": "SAR"}, "availability": object{"available": true}},
				{"id": "too-expensive", "price": object{"amount": 190000, "currency": "SAR"}, "availability": object{"available": true}},
				{"id": "sold-out", "price": object{"amount": 100000, "currency": "SAR"}, "availability": object{"available": false}},
			}}}}
		case "create_cart", "update_cart":
			writes = append(writes, request.Params.Arguments)
		}
		_ = json.NewEncoder(w).Encode(object{"jsonrpc": "2.0", "id": request.ID, "result": object{"content": []any{}, "structuredContent": result}})
	}))
	defer upstream.Close()
	c := &Commerce{api: &API{client: upstream.Client()}, endpoint: upstream.URL, profile: "https://example.com/profile"}
	s := &Session{}
	ctx := context.Background()
	if _, err := c.search(ctx, s, SearchInput{Query: "Phone", MaxPrice: floatPointer(1500)}); err != nil {
		t.Fatal(err)
	}
	if len(s.Products) != 1 || len(s.Products[0].Variants) != 1 {
		t.Fatal("availability/budget filters failed")
	}
	if _, err := c.add(ctx, s, AddInput{VariantID: "invented", Quantity: 1}); err == nil {
		t.Fatal("accepted invented variant")
	}
	for _, quantity := range []int{1, 2} {
		if _, err := c.add(ctx, s, AddInput{VariantID: "variant-1", Quantity: quantity}); err != nil {
			t.Fatal(err)
		}
	}
	if s.CheckoutURL != "https://sa.revibe.me/cart/test" {
		t.Fatal("addition did not expose continue_url immediately")
	}
	cart := writes[1]["cart"].(map[string]any)
	if cart["line_items"].([]any)[0].(map[string]any)["quantity"] != float64(3) {
		t.Fatal("existing quantity was not preserved")
	}
	if cart["buyer"] == nil || cart["discounts"] == nil {
		t.Fatal("cart replacement lost writable fields")
	}
	if _, err := c.checkout(ctx, s); err != nil {
		t.Fatal(err)
	}
	if s.CheckoutURL != "https://sa.revibe.me/cart/test" {
		t.Fatal("continue_url not used")
	}
	want := []string{"search_catalog", "create_cart", "get_cart", "update_cart", "get_cart"}
	if strings.Join(toolNames, ",") != strings.Join(want, ",") {
		t.Fatalf("wrong tool lifecycle: %v", toolNames)
	}
	if _, err := merchantURL("https://evil.example/pay"); err == nil {
		t.Fatal("accepted unexpected checkout host")
	}
}

func TestAgentToolLoop(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			writeModelStream(w, []object{{"type": "function_call", "call_id": "call-1", "name": "prepare_checkout", "arguments": "{}"}}, "")
			return
		}
		var request struct{ Input []object }
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		last := request.Input[len(request.Input)-1]
		if last["call_id"] != "call-1" || !strings.Contains(last["output"].(string), "add a device first") {
			t.Error("tool result was not returned to the model")
		}
		writeModelStream(w, []object{{"type": "message", "content": []object{{"type": "output_text", "text": "Choose a device first."}}}}, "Choose a device first.")
	}))
	defer upstream.Close()
	a := &Agent{api: &API{client: upstream.Client()}, commerce: &Commerce{}, key: "test-key", model: "test", endpoint: upstream.URL}
	s := &Session{}
	answer, err := a.chat(context.Background(), s, "checkout", func(string, any) error { return nil })
	if err != nil || answer != "Choose a device first." || calls != 2 || len(s.History) != 2 {
		t.Fatalf("unexpected result: %q, %v", answer, err)
	}
}

func TestHTTPValidationAndSessions(t *testing.T) {
	server := &Server{agent: &Agent{}, sessions: newSessionStore()}
	router := server.routes()
	request := func(body, origin string, cookie *http.Cookie) *httptest.ResponseRecorder {
		r := httptest.NewRequest("POST", "/api/chat", strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if cookie != nil {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, r)
		return w
	}
	if request(`{`, "", nil).Code != 400 {
		t.Fatal("invalid JSON accepted")
	}
	if request(`{"message":"hello"}`, "https://evil.example", nil).Code != 403 {
		t.Fatal("cross-origin request accepted")
	}
	// Session middleware also runs for the earlier malformed-body request.
	baseline := len(server.sessions.items)
	first := request(`{"message":"hello"}`, "", nil)
	if first.Code != 502 || !strings.Contains(first.Body.String(), "OPENAI_API_KEY") {
		t.Fatal("missing key should produce setup instructions")
	}
	cookie := first.Result().Cookies()[0]
	request(`{"message":"hello"}`, "", cookie)
	if len(server.sessions.items) != baseline+1 {
		t.Fatal("cookie did not preserve session")
	}
	request(`{"message":"hello"}`, "", nil)
	if len(server.sessions.items) != baseline+2 {
		t.Fatal("browsers share a session")
	}
}

func floatPointer(value float64) *float64 { return &value }

func TestChatHandlerFlushesBeforeModelFinishes(t *testing.T) {
	delivered := make(chan struct{})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: " + jsonText(object{"type": "response.output_text.delta", "delta": "Hello"}) + "\n\n"))
		w.(http.Flusher).Flush()
		select {
		case <-delivered:
		case <-r.Context().Done():
			return
		}
		writeModelStream(w, []object{{"type": "message", "content": []object{{"type": "output_text", "text": "Hello"}}}}, "")
	}))
	defer upstream.Close()
	agent := &Agent{api: &API{client: upstream.Client()}, key: "test", model: "test", endpoint: upstream.URL}
	server := &Server{agent: agent, sessions: newSessionStore()}
	backend := httptest.NewServer(server.routes())
	defer backend.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "POST", backend.URL+"/api/chat", strings.NewReader(`{"message":"hello"}`))
	req.Header.Set("Content-Type", "application/json")
	response, err := backend.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if !strings.HasPrefix(response.Header.Get("Content-Type"), "text/event-stream") {
		t.Fatal("not an SSE response")
	}
	emptyEvents := 0
	err = readSSE(response.Body, func(data []byte) error {
		var event object
		if err := json.Unmarshal(data, &event); err != nil {
			return err
		}
		if event["text"] == "Hello" {
			close(delivered)
		}
		// Only started and done carry empty objects; both must arrive.
		if len(event) == 0 {
			emptyEvents++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-delivered:
	default:
		t.Fatal("text was buffered until completion")
	}
	if emptyEvents != 2 {
		t.Fatal("missing terminal event")
	}
}

func writeModelStream(w http.ResponseWriter, output []object, text string) {
	w.Header().Set("Content-Type", "text/event-stream")
	if text != "" {
		_, _ = w.Write([]byte("data: " + jsonText(object{"type": "response.output_text.delta", "delta": text}) + "\n\n"))
	}
	_, _ = w.Write([]byte("data: " + jsonText(object{"type": "response.completed", "response": object{"output": output}}) + "\n\n"))
}

func TestRemovedShoppingRoutes(t *testing.T) {
	router := (&Server{agent: &Agent{}, sessions: newSessionStore()}).routes()
	for _, route := range []struct{ method, path string }{
		{"POST", "/api/products/search"}, {"GET", "/api/cart"},
		{"POST", "/api/cart/items"}, {"POST", "/api/checkout"},
	} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(route.method, route.path, strings.NewReader(`{}`)))
		if w.Code != 404 {
			t.Errorf("%s %s: got %d", route.method, route.path, w.Code)
		}
	}
}

func TestAIRefreshesCartAndStreamsContinuation(t *testing.T) {
	modelCalls, merchantCalls := 0, 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/model" {
			modelCalls++
			if modelCalls == 1 {
				if merchantCalls != 0 {
					t.Error("merchant called before model")
				}
				writeModelStream(w, []object{{"type": "function_call", "call_id": "refresh-1", "name": "get_cart", "arguments": "{}"}}, "")
				return
			}
			var body struct{ Input []object }
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			last := body.Input[len(body.Input)-1]
			if last["call_id"] != "refresh-1" || !strings.Contains(last["output"].(string), "SAR") {
				t.Error("merchant state not returned to AI")
			}
			writeModelStream(w, []object{{"type": "message", "content": []object{{"type": "output_text", "text": "Your cart is current."}}}}, "Your cart is current.")
			return
		}
		var request struct {
			ID     any    `json:"id"`
			Method string `json:"method"`
			Params struct{ Name string }
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch request.Method {
		case "initialize":
			_ = json.NewEncoder(w).Encode(object{"jsonrpc": "2.0", "id": request.ID, "result": object{"protocolVersion": "2025-06-18", "capabilities": object{"tools": object{}}, "serverInfo": object{"name": "test", "version": "1.0"}}})
		case "notifications/initialized":
			w.WriteHeader(202)
		case "tools/call":
			merchantCalls++
			if modelCalls != 1 || request.Params.Name != "get_cart" {
				t.Error("unexpected merchant action")
			}
			_ = json.NewEncoder(w).Encode(object{"jsonrpc": "2.0", "id": request.ID, "result": object{"content": []any{}, "structuredContent": object{"id": "cart-1", "currency": "SAR", "line_items": []object{}, "continue_url": "https://sa.revibe.me/cart/test"}}})
		default:
			_ = json.NewEncoder(w).Encode(object{"jsonrpc": "2.0", "id": request.ID, "error": object{"code": -32601, "message": "method not found"}})
		}
	}))
	defer upstream.Close()
	commerce := &Commerce{api: &API{client: upstream.Client()}, endpoint: upstream.URL + "/mcp"}
	agent := &Agent{api: commerce.api, commerce: commerce, key: "test", model: "test", endpoint: upstream.URL + "/model"}
	session := &Session{Cart: Cart{"id": "cart-1"}}
	continuation := false
	_, err := agent.chat(context.Background(), session, "Please check my cart", func(event string, data any) error {
		if event == "checkout_ready" {
			continuation = data.(object)["url"] == "https://sa.revibe.me/cart/test"
			if modelCalls != 1 {
				t.Error("continuation was delayed until model completion")
			}
		}
		return nil
	})
	if err != nil || modelCalls != 2 || merchantCalls != 1 || !continuation {
		t.Fatalf("unexpected flow: error=%v model=%d merchant=%d continuation=%v", err, modelCalls, merchantCalls, continuation)
	}
}
