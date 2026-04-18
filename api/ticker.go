package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	contentTypeSVG = "image/svg+xml"
	cacheControl   = "public, max-age=7200, s-maxage=7200"
	panelWidth     = 640
	panelHeight    = 380
)

type assetKind string

const (
	assetMetal  assetKind = "metal"
	assetCrypto assetKind = "crypto"
	assetEquity assetKind = "equity"
)

type assetSpec struct {
	Symbol           string
	DisplayName      string
	Kind             assetKind
	CoinGeckoID      string
	TwelveDataSymbol string
}

type assetQuote struct {
	Price     *float64
	Change24h *float64
}

type renderedAsset struct {
	Symbol     string
	PriceText  string
	ChangeText string
	ChangeFill string
}

type tickerView struct {
	Assets          []renderedAsset
	StatusLine      string
	StatusLineFill  string
	RenderedAtLabel string
}

var trackedAssets = []assetSpec{
	{Symbol: "GOLD", DisplayName: "Gold", Kind: assetMetal, TwelveDataSymbol: "XAU/USD"},
	{Symbol: "BTC", DisplayName: "Bitcoin", Kind: assetCrypto, CoinGeckoID: "bitcoin"},
	{Symbol: "ADA", DisplayName: "Cardano", Kind: assetCrypto, CoinGeckoID: "cardano"},
	{Symbol: "ROSE", DisplayName: "Oasis", Kind: assetCrypto, CoinGeckoID: "oasis-network"},
	{Symbol: "AVAX", DisplayName: "Avalanche", Kind: assetCrypto, CoinGeckoID: "avalanche-2"},
	{Symbol: "TSLA", DisplayName: "Tesla", Kind: assetEquity, TwelveDataSymbol: "TSLA"},
	{Symbol: "IONQ", DisplayName: "IonQ", Kind: assetEquity, TwelveDataSymbol: "IONQ"},
	{Symbol: "VTI", DisplayName: "Vanguard Total Stock Market ETF", Kind: assetEquity, TwelveDataSymbol: "VTI"},
}

var httpClient = &http.Client{
	Timeout: 8 * time.Second,
}

func Handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	view := loadTickerView(r.Context(), time.Now().UTC())
	svg := renderSVG(view)

	w.Header().Set("Content-Type", contentTypeSVG)
	w.Header().Set("Cache-Control", cacheControl)
	w.WriteHeader(http.StatusOK)

	if r.Method == http.MethodHead {
		return
	}

	_, _ = w.Write([]byte(svg))
}

func loadTickerView(ctx context.Context, now time.Time) tickerView {
	var (
		cryptoQuotes map[string]assetQuote
		stockQuotes  map[string]assetQuote
		goldQuotes   map[string]assetQuote
		cryptoErr    error
		stockErr     error
		goldErr      error
	)

	var wg sync.WaitGroup
	wg.Add(3)

	go func() {
		defer wg.Done()
		cryptoQuotes, cryptoErr = fetchCryptoQuotes(ctx)
	}()

	go func() {
		defer wg.Done()
		stockQuotes, stockErr = fetchStockQuotes(ctx)
	}()

	go func() {
		defer wg.Done()
		goldQuotes, goldErr = fetchGoldQuote(ctx)
	}()

	wg.Wait()

	quotesBySymbol := make(map[string]assetQuote, len(trackedAssets))
	providerFailures := make([]string, 0, 3)

	if cryptoErr != nil {
		providerFailures = append(providerFailures, "crypto")
	}
	for symbol, quote := range cryptoQuotes {
		quotesBySymbol[symbol] = quote
	}

	if stockErr != nil {
		providerFailures = append(providerFailures, "stocks")
	}
	for symbol, quote := range stockQuotes {
		quotesBySymbol[symbol] = quote
	}

	if goldErr != nil {
		providerFailures = append(providerFailures, "gold")
	}
	for symbol, quote := range goldQuotes {
		quotesBySymbol[symbol] = quote
	}

	rendered := make([]renderedAsset, 0, len(trackedAssets))
	for _, asset := range trackedAssets {
		quote := quotesBySymbol[asset.Symbol]
		changeText, changeFill := formatChange(quote.Change24h)
		rendered = append(rendered, renderedAsset{
			Symbol:     asset.Symbol,
			PriceText:  formatPrice(quote.Price),
			ChangeText: changeText,
			ChangeFill: changeFill,
		})
	}

	statusLine := "# source status: live snapshot"
	statusFill := "#8B949E"
	if len(providerFailures) > 0 {
		sort.Strings(providerFailures)
		statusLine = fmt.Sprintf("# partial data: %s unavailable", strings.Join(providerFailures, ", "))
		statusFill = "#D29922"
	}

	return tickerView{
		Assets:          rendered,
		StatusLine:      statusLine,
		StatusLineFill:  statusFill,
		RenderedAtLabel: now.Format("2006-01-02 15:04 UTC"),
	}
}

func fetchCryptoQuotes(ctx context.Context) (map[string]assetQuote, error) {
	ids := make([]string, 0, len(trackedAssets))
	symbolByID := make(map[string]string)
	for _, asset := range trackedAssets {
		if asset.Kind != assetCrypto {
			continue
		}
		ids = append(ids, asset.CoinGeckoID)
		symbolByID[asset.CoinGeckoID] = asset.Symbol
	}

	endpoint, err := url.Parse("https://api.coingecko.com/api/v3/simple/price")
	if err != nil {
		return nil, err
	}

	query := endpoint.Query()
	query.Set("ids", strings.Join(ids, ","))
	query.Set("vs_currencies", "usd")
	query.Set("include_24hr_change", "true")
	endpoint.RawQuery = query.Encode()

	var response map[string]struct {
		USD          *float64 `json:"usd"`
		USD24hChange *float64 `json:"usd_24h_change"`
	}
	if err := fetchJSON(ctx, endpoint.String(), &response); err != nil {
		return nil, err
	}

	quotes := make(map[string]assetQuote, len(response))
	for id, payload := range response {
		symbol, ok := symbolByID[id]
		if !ok {
			continue
		}
		quotes[symbol] = assetQuote{
			Price:     payload.USD,
			Change24h: payload.USD24hChange,
		}
	}

	return quotes, nil
}

func fetchStockQuotes(ctx context.Context) (map[string]assetQuote, error) {
	symbols := make([]string, 0, len(trackedAssets))
	allowed := make(map[string]string)
	for _, asset := range trackedAssets {
		if asset.Kind != assetEquity {
			continue
		}
		symbols = append(symbols, asset.TwelveDataSymbol)
		allowed[asset.TwelveDataSymbol] = asset.Symbol
	}

	return fetchTwelveDataQuotes(ctx, symbols, allowed)
}

func fetchGoldQuote(ctx context.Context) (map[string]assetQuote, error) {
	allowed := map[string]string{"XAU/USD": "GOLD"}

	quotes, err := fetchTwelveDataQuotes(ctx, []string{"XAU/USD"}, allowed)
	if err == nil {
		return quotes, nil
	}

	// Gold API gives us a no-auth fallback for price if the keyed provider is unavailable.
	fallbackQuotes, fallbackErr := fetchGoldAPIQuote(ctx)
	if fallbackErr == nil {
		return fallbackQuotes, nil
	}

	return nil, fmt.Errorf("twelvedata: %w; gold-api: %v", err, fallbackErr)
}

func fetchTwelveDataQuotes(ctx context.Context, symbols []string, allowed map[string]string) (map[string]assetQuote, error) {
	apiKey := strings.TrimSpace(os.Getenv("TWELVEDATA_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("TWELVEDATA_API_KEY is not set")
	}

	endpoint, err := url.Parse("https://api.twelvedata.com/quote")
	if err != nil {
		return nil, err
	}

	query := endpoint.Query()
	query.Set("symbol", strings.Join(symbols, ","))
	query.Set("apikey", apiKey)
	endpoint.RawQuery = query.Encode()

	var response twelveDataQuoteResponse
	if err := fetchJSON(ctx, endpoint.String(), &response); err != nil {
		return nil, err
	}

	if response.Code != "" {
		return nil, fmt.Errorf("twelvedata error %s: %s", response.Code, response.Message)
	}

	quotes := make(map[string]assetQuote, len(response.Quotes))
	for _, payload := range response.Quotes {
		assetSymbol, ok := allowed[payload.Symbol]
		if !ok {
			continue
		}

		price, err := parseNumericString(payload.Close)
		if err != nil {
			continue
		}
		change, err := parseNumericString(payload.PercentChange)
		if err != nil {
			change = nil
		}

		quotes[assetSymbol] = assetQuote{
			Price:     price,
			Change24h: change,
		}
	}

	return quotes, nil
}

func fetchGoldAPIQuote(ctx context.Context) (map[string]assetQuote, error) {
	var response struct {
		Price *float64 `json:"price"`
	}

	if err := fetchJSON(ctx, "https://api.gold-api.com/price/XAU", &response); err != nil {
		return nil, err
	}

	return map[string]assetQuote{
		"GOLD": {
			Price: response.Price,
		},
	}, nil
}

func fetchJSON(ctx context.Context, endpoint string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "gh-terminal/1.0 (+https://github.com/brudnak/gh-terminal)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("upstream returned %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if err := decodeJSON(body, target); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}

type twelveDataQuotePayload struct {
	Symbol        string `json:"symbol"`
	Close         string `json:"close"`
	PercentChange string `json:"percent_change"`
}

type twelveDataQuoteResponse struct {
	Quotes  []twelveDataQuotePayload
	Code    string
	Message string
}

func (r *twelveDataQuoteResponse) UnmarshalJSON(data []byte) error {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(data, &object); err != nil {
		return err
	}

	if rawCode, ok := object["code"]; ok {
		var codeString string
		if err := json.Unmarshal(rawCode, &codeString); err == nil {
			r.Code = codeString
		} else {
			var codeNumber int
			if err := json.Unmarshal(rawCode, &codeNumber); err == nil {
				r.Code = strconv.Itoa(codeNumber)
			}
		}
		_ = json.Unmarshal(object["message"], &r.Message)
		return nil
	}

	if _, ok := object["symbol"]; ok {
		var single twelveDataQuotePayload
		if err := json.Unmarshal(data, &single); err != nil {
			return err
		}
		r.Quotes = []twelveDataQuotePayload{single}
		return nil
	}

	r.Quotes = make([]twelveDataQuotePayload, 0, len(object))
	for _, raw := range object {
		var quote twelveDataQuotePayload
		if err := json.Unmarshal(raw, &quote); err != nil {
			return err
		}
		r.Quotes = append(r.Quotes, quote)
	}

	return nil
}

func decodeJSON(data []byte, target any) error {
	return json.Unmarshal(data, target)
}

func renderSVG(view tickerView) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="terminal-ticker live market feed">`, panelWidth, panelHeight, panelWidth, panelHeight))
	b.WriteString(`<defs>
  <linearGradient id="panel-bg" x1="0" y1="0" x2="0" y2="1">
    <stop offset="0%" stop-color="#0D1117"/>
    <stop offset="100%" stop-color="#0A0F14"/>
  </linearGradient>
</defs>`)
	b.WriteString(`<rect width="100%" height="100%" fill="url(#panel-bg)"/>`)
	b.WriteString(`<rect x="16" y="16" width="608" height="348" rx="12" fill="#0D1117" stroke="#30363D"/>`)
	b.WriteString(`<rect x="16" y="16" width="608" height="34" rx="12" fill="#161B22"/>`)
	b.WriteString(`<rect x="16" y="38" width="608" height="326" fill="#0D1117"/>`)
	b.WriteString(`<circle cx="40" cy="33" r="5" fill="#F85149"/>`)
	b.WriteString(`<circle cx="58" cy="33" r="5" fill="#D29922"/>`)
	b.WriteString(`<circle cx="76" cy="33" r="5" fill="#3FB950"/>`)
	b.WriteString(`<g font-family="ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,Liberation Mono,monospace" font-size="14" fill="#C9D1D9">`)
	b.WriteString(`<text x="44" y="84" fill="#3FB950">[~]</text>`)
	b.WriteString(`<text x="82" y="84">$</text>`)
	b.WriteString(`<text x="100" y="84">./terminal-ticker --live</text>`)
	b.WriteString(fmt.Sprintf(`<text x="596" y="84" text-anchor="end" fill="#8B949E">%s</text>`, escape(view.RenderedAtLabel)))
	b.WriteString(`<line x1="40" y1="102" x2="600" y2="102" stroke="#30363D"/>`)
	b.WriteString(`<text x="44" y="126" fill="#8B949E">ASSET</text>`)
	b.WriteString(`<text x="408" y="126" text-anchor="end" fill="#8B949E">PRICE</text>`)
	b.WriteString(`<text x="576" y="126" text-anchor="end" fill="#8B949E">24H</text>`)
	b.WriteString(`<line x1="40" y1="142" x2="600" y2="142" stroke="#30363D"/>`)

	rowY := 166
	for _, asset := range view.Assets {
		b.WriteString(fmt.Sprintf(`<text x="44" y="%d">%s</text>`, rowY, escape(asset.Symbol)))
		b.WriteString(fmt.Sprintf(`<text x="408" y="%d" text-anchor="end" fill="%s">%s</text>`, rowY, priceFill(asset.PriceText), escape(asset.PriceText)))
		b.WriteString(fmt.Sprintf(`<text x="576" y="%d" text-anchor="end" fill="%s">%s</text>`, rowY, asset.ChangeFill, escape(asset.ChangeText)))
		rowY += 24
	}

	b.WriteString(`<line x1="40" y1="340" x2="600" y2="340" stroke="#30363D"/>`)
	b.WriteString(fmt.Sprintf(`<text x="44" y="362" fill="%s">%s</text>`, view.StatusLineFill, escape(view.StatusLine)))
	b.WriteString(`</g></svg>`)

	return b.String()
}

func formatPrice(value *float64) string {
	if value == nil {
		return "N/A"
	}

	absValue := math.Abs(*value)
	switch {
	case absValue >= 1:
		return "$ " + formatFixed(*value, 2)
	case absValue >= 0.1:
		return "$ " + trimFixed(formatFixed(*value, 4), 2)
	default:
		return "$ " + formatFixed(*value, 4)
	}
}

func formatChange(value *float64) (string, string) {
	if value == nil {
		return "N/A", "#8B949E"
	}

	switch {
	case *value > 0:
		return fmt.Sprintf("▲ +%.2f%%", *value), "#3FB950"
	case *value < 0:
		return fmt.Sprintf("▼ %.2f%%", *value), "#F85149"
	default:
		return "• +0.00%", "#8B949E"
	}
}

func formatFixed(value float64, precision int) string {
	formatted := fmt.Sprintf("%.*f", precision, value)
	if !strings.Contains(formatted, ".") {
		return addCommas(formatted)
	}

	parts := strings.SplitN(formatted, ".", 2)
	return addCommas(parts[0]) + "." + parts[1]
}

func trimFixed(formatted string, minDecimals int) string {
	parts := strings.SplitN(formatted, ".", 2)
	if len(parts) != 2 {
		return formatted
	}

	decimals := parts[1]
	for len(decimals) > minDecimals && strings.HasSuffix(decimals, "0") {
		decimals = decimals[:len(decimals)-1]
	}

	return parts[0] + "." + decimals
}

func addCommas(raw string) string {
	negative := strings.HasPrefix(raw, "-")
	if negative {
		raw = strings.TrimPrefix(raw, "-")
	}

	if len(raw) <= 3 {
		if negative {
			return "-" + raw
		}
		return raw
	}

	pre := len(raw) % 3
	if pre == 0 {
		pre = 3
	}

	var b strings.Builder
	if negative {
		b.WriteByte('-')
	}

	b.WriteString(raw[:pre])
	for i := pre; i < len(raw); i += 3 {
		b.WriteByte(',')
		b.WriteString(raw[i : i+3])
	}

	return b.String()
}

func priceFill(text string) string {
	if text == "N/A" {
		return "#8B949E"
	}
	return "#C9D1D9"
}

func parseNumericString(raw string) (*float64, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("empty numeric string")
	}

	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, err
	}

	return &value, nil
}

func escape(value string) string {
	return html.EscapeString(value)
}
