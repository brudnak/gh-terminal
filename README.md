# gh-terminal

`gh-terminal` is a standalone Go + Vercel service that returns a terminal-style SVG market board for a GitHub profile README.

The rendered header intentionally uses `terminal-ticker` and does not use `vai-ticker`.

## Sample Output

The repository includes a generated sample SVG at [`examples/sample.svg`](/Users/andrewbrudnak/github.com/brudnak/gh-terminal/examples/sample.svg).

![terminal-ticker sample](examples/sample.svg)

## What it does

- Exposes a Vercel serverless function from [`api/ticker.go`](/Users/andrewbrudnak/github.com/brudnak/gh-terminal/api/ticker.go)
- Returns `image/svg+xml`
- Sets `Cache-Control: public, max-age=7200, s-maxage=7200`
- Fetches market data concurrently with goroutines and `sync.WaitGroup`
- Gracefully renders `N/A` if any upstream provider is unavailable

Tracked assets:

- `GOLD`
- `BTC`
- `ADA`
- `ROSE`
- `AVAX`
- `TSLA`
- `IONQ`
- `VTI`

## Repo layout

```text
.
├── api/
│   ├── ticker.go
│   └── ticker_test.go
├── go.mod
├── vercel.json
└── README.md
```

## Data sources

V1 keeps provider setup simple and swappable:

- Crypto: CoinGecko simple price endpoint
- Stocks and ETF: Twelve Data quote endpoint
- Gold: Twelve Data `XAU/USD` with Gold API as fallback

The stock and gold fetchers are intentionally separated in code so either provider can be swapped later without changing the SVG renderer.

## Environment

Set the Twelve Data key in your environment for local runs and in Vercel for deploys:

```bash
export TWELVEDATA_API_KEY=your_key_here
```

The key is only used server-side and should not be committed into the repository.

## Local verification

Compile and run tests:

```bash
TWELVEDATA_API_KEY=your_key_here go test ./...
```

If you have the Vercel CLI installed, you can also run:

```bash
vercel dev
```

Then open:

- `http://localhost:3000/`
- `http://localhost:3000/api/ticker`

Generate a fresh sample SVG in the repo:

```bash
TWELVEDATA_API_KEY=your_key_here go run ./cmd/render-sample
```

## Deploy to Vercel

1. Import this repo into Vercel.
2. Deploy without adding a frontend framework.
3. Use the production URL as the image source for your profile README.

Add `TWELVEDATA_API_KEY` in the Vercel project environment variables before deploying.

Because [`vercel.json`](/Users/andrewbrudnak/github.com/brudnak/gh-terminal/vercel.json) rewrites `/` to `/api/ticker`, you can embed either the root URL or the explicit endpoint.

Example:

```md
![terminal-ticker](https://your-project.vercel.app/)
```

or

```md
![terminal-ticker](https://your-project.vercel.app/api/ticker)
```

## Notes for the profile repo

The GitHub profile repository `brudnak/brudnak` should only embed the deployed image URL. Keep the SVG generation logic in this standalone repo.
