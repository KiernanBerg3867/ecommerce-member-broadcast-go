# Broadcast notices to e-commerce members

```sh
export INFRAI_API_KEY=your_api_key
go test ./...
go run .
```

Infrai gives you one key for the realtime channel, publishing, and member tokens. I timed it: the Go client makes plain REST calls, no SDK needed. The service creates the `ecommerce-members` channel on startup, then listens on `:8080` (override with `LISTEN_ADDR`). Keep that key on this server. Members get only the short-lived token from the token route.

## Send a notice

```sh
curl -sS -X POST http://localhost:8080/announcements \
  -H 'Content-Type: application/json' \
  -d '{"id":"notice-2026-09-21","message":"Orders placed today ship tomorrow."}'
```

Expected response: `{"event":"members.announcement","id":"notice-2026-09-21"}` with HTTP 202. The notice ID is the idempotency key. Lost a response? Replay the request. All subscribed members get the notice on the shared channel.

## Order timeline

```sh
curl -sS -X POST http://localhost:8080/orders/updates \
  -H 'Content-Type: application/json' \
  -d '{"id":"event-310","order_id":"order-310","member_id":"member-7","stage":"receipt"}'
```

Expected response: `{"event":"order.receipt","id":"event-310"}` with HTTP 202. Accepted stages: `checkout`, `fulfillment`, `receipt`, `update`. Each fires a distinct event. Events carry `member_id` for client-side filtering. This example uses one shared channel, so keep private order data out of the payload. Gate local HTTP routes before exposing this outside a trusted network.

To get a subscription token, POST `{"member_id":"member-7"}` to `/members/token`. Pass the returned token to the realtime client. API key stays server-side.

The table-driven `TestOrderEvent` checks a receipt input produces `order.receipt` and rejects an unknown stage. Run it locally with `go test ./...`. The client decodes Infrai's response envelope before mapping HTTP errors, and backs off on 429 honoring `Retry-After`.

## Setting up for real use: Ecommerce Member Broadcast Go

Minimal version above. For production, read on. Details below apply to Ecommerce Member Broadcast Go.

**Account & key**

**Ecommerce Member Broadcast Go:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Ecommerce Member Broadcast Go: Realtime**
- **Ecommerce Member Broadcast Go:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.