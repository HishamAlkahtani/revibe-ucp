# Revibe UCP chat

A small **Gin backend + Next.js frontend**. Go uses Gin for HTTP routes and the official MCP Go SDK for merchant tool calls. Next uses plain JavaScript and CSS.

## Run

Requires Go 1.25+ and Node 22.13+.

From the project root:

```powershell
Copy-Item .env.example .env
# Set OPENAI_API_KEY in .env.
npm install
cd backend
go mod tidy
cd ..
npm run backend
```

In another terminal, from the same project root:

```powershell
npm run dev
```

Open http://127.0.0.1:3000. Try **Find an iPhone 13 under 1500 SAR**, choose the variant in chat, and confirm when the assistant asks whether to add it. Use the merchant continuation link to review and pay.

Next runs on port 3000. Go runs on port 8080. Next forwards `/api/*` to Go, including the session cookie. The API key stays in Go.

## Read the code

1. `app/page.js`: pre-rendered server shell. `app/chat.js`: interactive React chat, variants and cart.
2. `next.config.mjs`: forwards `/api/*` to Go; contains no commerce logic.
3. `backend/routes.go`: every endpoint and its named Gin handler in one place. `chat_handler.go` streams the conversation and structured shopping state.
4. `backend/agent.go`: OpenAI tool loop — ask the model, run its tools, return their results, repeat until it answers.
5. `backend/commerce.go`: product and cart rules executed only by AI-requested tools.
6. `backend/mcp.go`: UCP discovery and the official MCP client. The SDK handles initialization, JSON-RPC and JSON/SSE transport.
7. `backend/session.go`: browser sessions. `backend/main.go`: wires dependencies and starts Gin.

**Browser → Next proxy → Go → OpenAI tool call → Revibe UCP → Go → browser.**

All merchant operations run through AI-requested tools. Product variants and cart summaries are read-only; buyers make choices and confirm additions in chat. The merchant continuation link is the only manual shopping action.

| Endpoint | Handler | Purpose |
| --- | --- | --- |
| `GET /api/health` | `healthHandler` | Backend liveness |
| `POST /api/chat` | `chatHandler` | AI conversation |

The frontend is **pre-rendered HTML plus client-side interactivity**, not a client-only blank page. Live shopping state is fetched in the browser. Session state, AI credentials and commerce actions stay in Go.

Chat uses **HTTP POST with an SSE response**: text appears as it is generated, and successful tools emit product, cart and checkout events immediately. The client uses streaming fetch, not WebSockets. The health endpoint returns JSON. MCP's potential use of SSE is a separate Go-to-merchant connection. See `docs/streaming.md` for the event contract.

Use **Stop** to cancel a turn. Cancellation propagates to Go's upstream requests, but cannot undo a cart action already accepted by Revibe. Ask the assistant to check the cart before retrying; its `get_cart` tool reads the authoritative merchant state. Requests are never replayed automatically.

## Checkout behavior

The backend uses the cart lifecycle: `create_cart` on the first addition, then `get_cart` followed by `update_cart` for later additions. The AI can also call `get_cart` to refresh the basket or `prepare_checkout` to refresh and validate the continuation link. There is no `USE_CART` mode switch.

A successful addition immediately exposes the validated `continue_url` in structured state, so the buyer does not need a separate checkout command. If no valid URL is returned, the assistant can try `prepare_checkout`; the UI never invents a link.

The button opens the resource's **`continue_url`**, as returned by Revibe. Creating a cart does not place an order or take payment. The buyer finishes at the merchant's checkout. This app does not track payment success.

Catalog and Cart MCP calls are anonymous: no merchant token or buyer-IP header is sent. This app uses the cart continuation link for the buyer to finish in the browser; it does not call Checkout MCP. Shopify permits anonymous catalog/cart calls at lower rate limits. See [Shopify Cart MCP](https://shopify.dev/docs/agents/carts-and-checkout/cart-mcp).

Cart updates replace the full resource, so Go preserves existing lines and writable fields. SAR prices are halalas: `123900` means **SAR 1,239**.

## Checks

```powershell
npm test
cd backend
go vet ./...
cd ..
npm run build
```

Go tests cover the search/cart/checkout flow, cart replacement, the AI tool loop, input validation and browser session isolation. External services are mocked: no purchases or API credits.

Streaming-only checks, without Gin/MCP dependencies:

```powershell
npm run test:client
cd backend
go test http_client.go logging.go stream.go stream_test.go
```

These verify incremental delivery before model completion, fragmented SSE/UTF-8, incomplete streams and cancellation. Full tests and Next's build also require installing dependencies.

## Backend logs and API failures

Application logs go to stdout. From the project root, capture all output:

```powershell
npm run backend > backend.log 2>&1
```

Use `>> backend.log 2>&1` to append. To display and save together:

```powershell
npm run backend 2>&1 | Tee-Object -FilePath backend.log
```

Press Ctrl+C to stop. `2>&1` also captures Go compiler/runtime errors from stderr.

The `npm run backend` terminal shows model steps, actual tool payloads, tool durations, provider errors and one HTTP summary per request. No request IDs, provider IDs or duplicate request-start chatter. Stream failures are logged separately because an SSE response can have HTTP status 200 and still fail later.

With `LOG_TOOL_PAYLOADS=true` (the default), logs pretty-print both layers:

- **AI TOOL request/response**: the model's function name and parsed arguments, then the tool result.
- **MCP request/response**: the actual merchant tool, its `tools/call` argument structure and returned MCP result. This distinguishes `add_to_cart` (our app's tool) from `create_cart` (Revibe's actual tool).

For example, cart creation produces a request shaped like:

```json
{
  "method": "tools/call",
  "params": {
    "name": "create_cart",
    "arguments": {
      "cart": {
        "line_items": [{"item": {"id": "gid://shopify/ProductVariant/123"}, "quantity": 1}],
        "context": {"address_country": "SA", "currency": "SAR"}
      },
      "meta": {"ucp-agent": {"profile": "https://your-agent.example/.well-known/ucp"}}
    }
  }
}
```

Buyer/payment fields and private cart/checkout IDs are redacted. Continuation URL hostnames remain visible for the Shop-vs-Revibe experiment, but private paths and query values are redacted. Set `LOG_TOOL_PAYLOADS=false` for concise operational logs.

Provider errors appear in chat with their actual code. `credit_balance_exhausted` means the OpenAI account has no remaining credits; add credits at https://platform.openai.com/settings/organization/billing/ or configure a funded API project/key, then restart Go if the key changes. `max_output_tokens` is reported separately as an output-limit error.

## Deliberate limits

- Local demo: in-memory sessions expire after an hour; restarting Go clears them.
- Four AI tools: search products, add to cart, get cart and prepare checkout. No remove/edit tool or payment confirmation tracking.
- Chat needs an OpenAI API key; messages and relevant cart/catalog data are sent to OpenAI.
- Development uses Shopify's public example agent profile. Before production, host your own HTTPS UCP agent profile and set `UCP_AGENT_PROFILE_URL`; add authentication and rate limits.
- Both servers bind to localhost. The Go origin check expects the UI on port 3000.
- `.env` supports `KEY=value`, optional surrounding quotes and full-line comments. No variable interpolation.

References: [Revibe discovery](https://sa.revibe.me/.well-known/ucp), [Shopify Cart MCP](https://shopify.dev/docs/agents/carts-and-checkout/cart-mcp), [OpenAI function calling](https://developers.openai.com/api/docs/guides/function-calling), [Next.js setup](https://nextjs.org/docs/app/getting-started/installation).
