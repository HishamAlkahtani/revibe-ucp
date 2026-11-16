# Chat streaming

This app uses **HTTP POST with an SSE-formatted response**, consumed with `fetch` and a streaming reader.

`backend/stream.go` consumes OpenAI events. `backend/agent.go` runs complete tool calls. `backend/chat_handler.go` flushes application events. `app/stream.js` parses frames and `app/chat.js` updates React state. The health endpoint returns JSON; all merchant operations are AI tools reached through chat.

## Why

The client sends one message, then receives text and action updates. A streamed HTTP response fits that direction of communication. A persistent bidirectional socket is optional, not a prerequisite for token streaming.

OpenAI's Responses API supports SSE with `stream: true`. Go should consume those events, execute complete tool calls, then expose a small application-specific event contract to the client. Do not forward provider internals or incomplete tool arguments directly into the UI.

Application events:

| Event | Data |
| --- | --- |
| `started` | Headers flushed; the turn is running |
| `text_delta` | Visible assistant text to append |
| `tool_started` | Friendly action name, e.g. “Searching Revibe” |
| `tool_finished` | Tool name and success flag |
| `products` | Product cards returned by a completed search |
| `cart` | Authoritative merchant cart snapshot |
| `checkout_ready` | Validated merchant `continue_url` |
| `state` | Final product/cart/checkout snapshot, also sent before a terminal error |
| `done` | Turn finished |
| `error` | Turn failed; retain any confirmed cart state |

One POST remains open across the entire AI/tool loop. Gin flushes each event and sets `X-Accel-Buffering: no` and `Cache-Control: no-cache, no-transform`. Next's proxy timeout is five minutes; Go's turn timeout is four minutes. An actual Next/deployment streaming check is still required once dependencies are available. HTTP chunks are not event boundaries: both parsers buffer complete SSE events, including split UTF-8 characters.

Use `fetch`, rather than the browser's native `EventSource`, because the chat request carries a JSON POST body and may use an Authorization header. Native EventSource's constructor accepts a URL and credentials options, not arbitrary request bodies/headers. Fetch does not provide EventSource's automatic reconnection.

## Cancellation and mobile

- Browser cancellation uses AbortController. Go propagates request cancellation to model/tool calls. Canceling a connection does not undo a cart write already accepted by Revibe.
- After cancellation, ask the assistant to check the cart to read confirmed merchant state. No automatic retry or replay is performed. An HTTP success header alone does not signal turn success: consume `done` or `error`. EOF without either is a broken turn.
- A broken connection is not permission to repeat a shopping action. Refresh authoritative state before retrying mutations; add request IDs/idempotency handling before supporting automatic replay.
- On mobile, use a networking client that exposes streamed response bytes. Expo documents `expo/fetch` and native stream APIs; verify the app's exact runtime/version rather than assuming React Native's global fetch behaves identically to a browser.
- For background jobs that must survive disconnects, introduce persisted runs and resumable events later. Neither SSE nor WebSockets supplies durable execution by itself.

## When WebSockets make sense

Use a persistent socket when the application needs frequent bidirectional events or multiplexed realtime sessions. OpenAI also offers Responses WebSocket mode for tool-heavy workflows; that **backend-to-model** choice is independent of the **client-to-Go** protocol. It can be evaluated later without changing the client SSE contract.

For realtime voice, evaluate WebRTC; OpenAI recommends it for browser/mobile realtime audio. That is a different workload from this text checkout assistant.

## Sources

- [OpenAI response streaming](https://developers.openai.com/api/docs/guides/streaming-responses)
- [OpenAI Responses WebSocket mode](https://developers.openai.com/api/docs/guides/websocket-mode)
- [MDN: reading a fetch response incrementally](https://developer.mozilla.org/en-US/docs/Web/API/Fetch_API/Using_Fetch#streaming_the_response_body)
- [MDN: EventSource constructor](https://developer.mozilla.org/en-US/docs/Web/API/EventSource/EventSource)
- [Expo fetch and streams](https://docs.expo.dev/versions/latest/sdk/expo/)
- [OpenAI realtime WebRTC](https://developers.openai.com/api/docs/guides/realtime-webrtc)
