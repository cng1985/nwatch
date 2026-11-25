# Demo ERP — Sales & Inventory

Self-contained demo under `erp/` (does not change the existing NWatch app).

Stack:

- **Backend**: Go + Gin + GORM + SQLite
- **Frontend**: Vue 3 + Vue Router + Vite

Closed loop:

1. Products / SKUs (with seed data)
2. Inventory stock levels + stock movements
3. Sales orders: **create (draft) → confirm → ship/fulfill → deduct inventory**

## Prerequisites

- Go 1.22+
- Node.js 18+ and npm

## Run locally

Open two terminals from the repo root.

### 1. Backend (API on `:8088`)

```bash
cd erp/backend
go run ./cmd/server
```

Optional flags / env:

- `-addr :8088` (default)
- `-db /path/to/erp.db` or `ERP_DB=...` (default: `erp/backend/data/erp.db`)

Health check: `GET http://127.0.0.1:8088/api/health`

### 2. Frontend (UI on `:5178`)

```bash
cd erp/frontend
npm install
npm run dev
```

Open http://127.0.0.1:5178 — Vite proxies `/api` to the backend.

## Demo walkthrough

Seed data loads automatically on first start (5 products, stock, and one draft order).

1. **Products** — browse seeded SKUs (or add one).
2. **Inventory** — note on-hand quantities and seed movements.
3. **Sales Orders** — open the seeded draft (`Acme Office Supplies`), or create a new draft.
4. **Order detail** — click **Confirm**, then **Ship / Fulfill**.
5. Return to **Inventory** — stock decreased; a `sale_out` movement appears linked to the order.

Fulfill fails with a clear error if stock is insufficient.

## API sketch

| Method | Path | Notes |
|--------|------|--------|
| GET | `/api/products` | List products |
| POST | `/api/products` | Create product (+ optional `initial_qty`) |
| GET | `/api/inventory` | Stock levels |
| GET | `/api/inventory/movements` | Recent movements |
| GET | `/api/orders` | List orders |
| GET | `/api/orders/:id` | Order detail |
| POST | `/api/orders` | Create draft |
| POST | `/api/orders/:id/confirm` | draft → confirmed |
| POST | `/api/orders/:id/fulfill` | confirmed → fulfilled + stock out |

## Reset demo data

Stop the API and delete the SQLite file, then start again:

```bash
rm -f erp/backend/data/erp.db
```
