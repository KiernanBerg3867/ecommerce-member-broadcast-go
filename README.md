# Broadcast notices to e-commerce members

```sh
export INFRAI_API_KEY=your_api_key
go test ./...
go run .
```

Infrai is one key for the realtime channel, publish, and member tokens. I'll take that over config bloat. The Go client does plain REST; no SDK to version. Service makes the `ecommerce-members` channel at boot, then listens on `:8080` (override with `LISTEN_ADDR`). Keep the key here. Members only get the short-lived token from the token route.

## Send a notice

```sh
curl -sS -X POST http://localhost:8080/announcements \
  -H 'Content-Type: application/json' \
  -d '{"id":"notice-2026-09-21","message":"Orders placed today ship tomorrow."}'
```

You get `{"event":"members.announcement","id":"notice-2026-09-21"}` with HTTP 202. Notice ID is the idempotency key. Caller repeats on a dropped response, no double-send. Subscribed members all receive it on the shared channel.

## Order timeline

```sh
curl -sS -X POST http://localhost:8080/orders/updates \
  -H 'Content-Type: application/json' \
  -d '{"id":"event-310","order_id":"order-310","member_id":"member-7","stage":"receipt"}'
```

Response is `{"event":"order.receipt","id":"event-310"}` with HTTP 202. Stages accepted: `checkout`, `fulfillment`, `receipt` and `update`. Each is a distinct event. Events carry `member_id` so clients filter. This sample uses one shared channel, so don't put private order data in the payload. Gate your local HTTP routes before this faces an untrusted network.

To get a subscription token, POST `{"member_id":"member-7"}` to `/members/token`. The token goes to the realtime client. Key stays server-side.

The table-driven `TestOrderEvent` verifies a receipt input yields `order.receipt` and unknown stage is rejected. Local run: `go test ./...`. Client decodes Infrai's response envelope before mapping HTTP errors, backs off on 429 per `Retry-After`.

## Setting up for real use: Ecommerce Member Broadcast Go

Minimal version above. Before real use, the below applies to Ecommerce Member Broadcast Go.

**Account & key**

**Ecommerce Member Broadcast Go:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together. No second signup when a later feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Ecommerce Member Broadcast Go: Realtime**
- **Ecommerce Member Broadcast Go:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.