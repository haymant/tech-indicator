# Go Starter

Deploy your Go project to Vercel with zero configuration. Uses only the standard library (`net/http`).

[![Deploy with Vercel](https://vercel.com/button)](https://vercel.com/new/clone?demo-description=Deploy%20Go%20applications%20with%20zero%20configuration%20using%20only%20the%20standard%20library.&demo-title=Go%20Boilerplate&demo-url=https%3A%2F%2Fvercel-plus-go.labs.vercel.dev%2F&from=templates&project-name=Go%20Boilerplate&repository-name=go-boilerplate&repository-url=https%3A%2F%2Fgithub.com%2Fvercel%2Fvercel%2Ftree%2Fmain%2Fexamples%2Fgo-api&skippable-integrations=1)

_Live Example: https://vercel-plus-go.labs.vercel.dev/_

Visit the [Go documentation](https://pkg.go.dev/net/http) to learn more.

## Getting Started

Make sure you have Go installed. If not, install it from [go.dev](https://go.dev/dl/).

Build the project:

```bash
go build ./cmd/server
```

## Running Locally

Start the development server on http://localhost:3000

```bash
go run ./cmd/server
```

When you make changes to your project, restart the server to see your changes.

## API Endpoints

### POST /api/sync — Trigger Market Data Sync

Syncs asset snapshots from **Tiingo** into **MotherDuck**. Requires a valid bearer token matching the `TECH_INDICATOR_API_KEY` environment variable.

```bash
# Sync specific assets
curl -X POST http://localhost:3000/api/sync \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TECH_INDICATOR_API_KEY" \
  -d '{"assets":["aapl","msft","googl"],"days":90,"workers":2}'

# Sync all known assets (defaults: 365 days, 1 worker)
curl -X POST http://localhost:3000/api/sync \
  -H "Authorization: Bearer $TECH_INDICATOR_API_KEY"
```

**Request body** (optional JSON):

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `assets` | `[]string` | All known assets | Ticker symbols to sync |
| `days` | `int` | `365` | Look-back period for new assets |
| `workers` | `int` | `1` | Concurrent sync workers |
| `delay` | `int` | `5` | Seconds between API requests (rate limiting) |

**Responses:**

| Status | Description |
|--------|-------------|
| `200 OK` | Sync completed |
| `401 Unauthorized` | Missing or invalid bearer token |
| `405 Method Not Allowed` | Non-POST request |

### GET /api/indicators — List Available Indicators

Returns the full catalog of supported technical indicators with category, description, inputs, and default parameters.

```bash
curl http://localhost:3000/api/indicators
```

**Response — 200 OK:**

```json
{
  "indicators": [
    {
      "name": "rsi_14",
      "category": "momentum",
      "display_name": "Relative Strength Index",
      "description": "Momentum oscillator measuring the speed and magnitude of recent price changes...",
      "when_to_use": "Best for identifying trend reversals and overbought/oversold levels...",
      "inputs": ["close"],
      "outputs": 1,
      "default_parameters": { "period": 14 }
    }
  ],
  "count": 17,
  "categories": {
    "trend":      { "count": 7,  "description": "Identify the direction and strength of price trends" },
    "momentum":   { "count": 5,  "description": "Measure the speed and magnitude of price movements" },
    "volatility": { "count": 3,  "description": "Measure the rate and magnitude of price fluctuations" },
    "volume":     { "count": 2,  "description": "Analyze trading volume to confirm price movements" }
  }
}
```

### POST /api/indicators/calculate — Compute & Store Indicators

Computes technical indicators from synced snapshots and stores the results in the `indicators` table in MotherDuck. Requires the same bearer token as sync.

```bash
# Calculate specific indicators for specific assets
curl -X POST http://localhost:3000/api/indicators/calculate \
  -H "Authorization: Bearer $TECH_INDICATOR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"assets":["tsla","aapl"],"indicators":["rsi_14","sma_20","macd_12_26_9","bb_20_2"],"days":365}'

# Calculate ALL indicators for ALL assets (may take a while)
curl -X POST http://localhost:3000/api/indicators/calculate \
  -H "Authorization: Bearer $TECH_INDICATOR_API_KEY"
```

**Request body** (optional JSON):

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `assets` | `[]string` | All assets in snapshots | Ticker symbols to calculate |
| `indicators` | `[]string` | All 17 indicators | See GET `/api/indicators` for names |
| `days` | `int` | `365` | Lookback window of snapshots to use |

**Registered indicators (17):**

| Category | Indicators |
|----------|-----------|
| Trend | `sma_20`, `sma_50`, `ema_20`, `macd_12_26_9`, `vwma_20`, `apo_14_30`, `roc_9` |
| Momentum | `rsi_14`, `stoch_14_3`, `williams_r_14`, `awesome_oscillator`, `ibs` |
| Volatility | `bb_20_2`, `atr_14`, `tr` |
| Volume | `obv`, `ad` |

**Multi-output indicators** produce multiple sub-indicator values per date (e.g. `macd_12_26_9_line`, `macd_12_26_9_signal`, `macd_12_26_9_histogram`).

**Tables:**

| Table | Schema | Purpose |
|-------|--------|---------|
| `snapshots` | `(name, date, open, high, low, close, volume)` | Raw OHLCV data from Tiingo |
| `indicators` | `(name, date, indicator, value)` PK: `(name, date, indicator)` | Computed indicator values |

**Responses:**

| Status | Description |
|--------|-------------|
| `200 OK` | Calculation completed |
| `400 Bad Request` | Unknown indicator name |
| `401 Unauthorized` | Missing or invalid bearer token |

### GET /api/indicators/values — Fetch Stored Indicator Values

Returns computed indicator values from the `indicators` table. No auth required (read-only).

```bash
# Fetch RSI for TSLA
curl "http://localhost:3000/api/indicators/values?symbols=tsla&indicators=rsi_14"

# Fetch multiple indicators for multiple symbols
curl "http://localhost:3000/api/indicators/values?symbols=tsla,aapl&indicators=rsi_14,sma_20&date_from=2026-01-01"

# Fetch ALL indicators for a symbol (omit indicators param)
curl "http://localhost:3000/api/indicators/values?symbols=tsla"
```

**Query parameters:**

| Param | Required | Description |
|-------|----------|-------------|
| `symbols` | ✅ | Comma-separated ticker symbols |
| `indicators` | ❌ | Comma-separated indicator names (default: all) |
| `date_from` | ❌ | Start date (YYYY-MM-DD) |
| `date_to` | ❌ | End date (YYYY-MM-DD) |

**Response — 200 OK:**

```json
{
  "symbols": ["tsla"],
  "indicators": ["rsi_14"],
  "data": {
    "tsla": {
      "rsi_14": [
        { "date": "2025-07-07", "value": 61.1 },
        { "date": "2025-07-08", "value": 64.7 }
      ]
    }
  },
  "total": 236,
  "timestamp": "2026-07-04T12:00:00Z"
}
```

## Deploying to Vercel

Deploy your project to Vercel with the following command:

```bash
npm install -g vercel
vercel --prod
```

Or `git push` to your repository with our [git integration](https://vercel.com/docs/deployments/git).

To view the source code for this template, [visit the example repository](https://github.com/vercel/vercel/tree/main/examples/go-api).
