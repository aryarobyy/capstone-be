# History Module

Anomaly history for sensors. Each record is one anomaly episode: a time span during which a sensor `parameter` stayed outside its configured threshold.

Backed by the `anomalies` table ([migration](../../../migrations/000004_create_anomalies_table.up.sql)).

## How records get created

Most rows are written automatically by the alert evaluator ([internal/alert/evaluate.go](../../alert/evaluate.go)) on sensor reading ingest:

1. A reading goes out of range → the evaluator tracks the start time in `alert_states`.
2. Once it stays out of range for at least the rule's `duration` seconds → an anomaly row is inserted with status `WARNING`.
3. While it stays out of range → `last_detected_at` and `accumulated_duration` are updated.
4. It returns to range (or readings stop for longer than the rule's gap) → status becomes `NORMAL` and `resolved_at` is set.

Clearing an alert ([internal/alert/service.go](../../alert/service.go)) also marks the linked anomalies `NORMAL`.

`POST /api/history/create` exists for manual or test entries.

## Data model

| Field | Type | Notes |
|---|---|---|
| `id` | int64 | |
| `sensor_id` | int64 | |
| `parameter` | string | e.g. `ph`, `temperature` (max 50 chars) |
| `started_at` | timestamp | when the value first went out of range |
| `last_detected_at` | timestamp | defaults to `started_at` on create |
| `accumulated_duration` | int64 | seconds |
| `status` | enum | `FATAL`, `CRITICAL`, `WARNING`, `NORMAL` |
| `resolved_at` | timestamp \| null | null while the anomaly is still active |
| `created_at`, `updated_at` | timestamp | |

## Endpoints

All routes are `POST`, take a JSON body, and need `Authorization: Bearer <token>` (`JWTAuth` + `ResourceAccess` middleware).

Success response: `{ "success": true, "message": "...", "data": ... }`
Error response: `{ "success": false, "message": "...", "errors": ... }`

### `POST /api/history/create`

```json
{
  "sensor_id": 1,
  "parameter": "ph",
  "started_at": "2026-10-19T08:00:00Z",
  "last_detected_at": "2026-10-19T08:05:00Z",
  "accumulated_duration": 300,
  "status": "WARNING",
  "resolved_at": null
}
```

Required: `sensor_id`, `parameter`, `started_at`, `status`. Returns `201` with the created record.

### `POST /api/history/list`

```json
{
  "sensor_id": 1,
  "parameter": "ph",
  "status": "WARNING",
  "date": "2026-10-19",
  "limit": 10,
  "index": 0
}
```

Every filter is optional; send `{}` to get everything.

- `date` matches the calendar date of `started_at` (`YYYY-MM-DD`).
- `limit` defaults to 10, max 100.
- `index` is a row **offset**, not a page number (page 2 with limit 10 → `index: 10`).
- Sorted by `started_at DESC, id DESC`.

```json
{
  "success": true,
  "message": "History retrieved successfully",
  "data": {
    "data": [ { "id": 1, "sensor_id": 1, "parameter": "ph", "status": "WARNING", "...": "..." } ],
    "total": 42,
    "limit": 10,
    "index": 0
  }
}
```

### `POST /api/history/newest`

Returns the single most recent anomaly (by `started_at`). Both filters are optional.

```json
{ "sensor_id": 1, "parameter": "ph" }
```

Send `{}` for the newest across all sensors (an empty body is rejected). Returns `404` when nothing matches.

```json
{
  "success": true,
  "message": "Newest history retrieved successfully",
  "data": { "id": 42, "sensor_id": 1, "parameter": "ph", "status": "WARNING", "...": "..." }
}
```

### `POST /api/history/detail`

```json
{ "id": 1 }
```

### `POST /api/history/delete`

```json
{ "id": 1 }
```

Deleting an anomaly sets `alert_states.anomaly_id` to `NULL` (`ON DELETE SET NULL`). If the anomaly was still active, the evaluator will open a new one on the next out-of-range reading.

## Errors

| Status | When |
|---|---|
| `400` | invalid body / missing required field / unknown `status` |
| `401` | missing or invalid token |
| `404` | `id` not found (detail, delete), or no match (newest) |
| `500` | database error |

## Access control

`ResourceAccess` currently lets **any authenticated user** call every history endpoint, and `list` is not scoped to the user's own sensors. Admins and non-admins see the same data.

## Files

| File | Role |
|---|---|
| [router.go](router.go) | route registration and wiring |
| [handler.go](handler.go) | HTTP binding, error → status mapping |
| [service.go](service.go) | status validation, defaults, pagination clamping |
| [repository.go](repository.go) | SQL against `anomalies` |
| [dto.go](dto.go) | request/response structs |
| [entity.go](entity.go) | `Anomaly` model and `AnomalyStatus` enum |

## Tests

```bash
go test ./internal/modules/history/...
```
