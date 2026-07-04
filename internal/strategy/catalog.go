package strategy

import (
	"fmt"

	cindicator "github.com/cinar/indicator/v2/strategy"
	"github.com/cinar/indicator/v2/strategy/compound"
	"github.com/cinar/indicator/v2/strategy/momentum"
	"github.com/cinar/indicator/v2/strategy/trend"
	"github.com/cinar/indicator/v2/strategy/volatility"
	"github.com/cinar/indicator/v2/strategy/volume"
	cvolatility "github.com/cinar/indicator/v2/volatility"
)

// StrategyDef describes a strategy type that can be instantiated.
type StrategyDef struct {
	Type          string
	Name          string
	Category      string
	Description   string
	DefaultParams map[string]any
	Constructor   func(params map[string]any) (cindicator.Strategy, error)
}

// Catalog maps strategy type keys to their definitions.
var Catalog = map[string]StrategyDef{}

// CategoryInfo holds metadata for a category.
type CategoryInfo struct {
	Count       int
	Description string
}

// CategoryDescriptions maps category keys to human-readable descriptions.
var CategoryDescriptions = map[string]string{
	"base":       "Base and baseline strategies",
	"trend":      "Trend-following strategies",
	"momentum":   "Momentum-based strategies",
	"volatility": "Volatility-based strategies",
	"volume":     "Volume-based strategies",
	"compound":   "Compound/composite strategies",
	"decorator":  "Decorator/wrapper strategies",
}

var categoryOrder = []string{"base", "trend", "momentum", "volatility", "volume", "compound", "decorator"}

func init() {
	registerBase()
	registerTrend()
	registerMomentum()
	registerVolatility()
	registerVolume()
	registerCompound()
	registerDecorator()
}

// ─── Base Strategies ───────────────────────────────────────────────────────

func registerBase() {
	add("buy_and_hold_strategy", "Buy and Hold", "base",
		"Buy on the first signal and hold until the end. Baseline benchmark strategy.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return cindicator.NewBuyAndHoldStrategy(), nil
		})

	add("and_strategy", "AND Strategy", "base",
		"Buy only when ALL sub-strategies recommend Buy, Sell when ANY recommends Sell.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return nil, fmt.Errorf("and_strategy requires sub-strategies; use via compound strategies")
		})

	add("or_strategy", "OR Strategy", "base",
		"Buy when ANY sub-strategy recommends Buy, Sell when ALL recommend Sell.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return nil, fmt.Errorf("or_strategy requires sub-strategies; use via compound strategies")
		})

	add("majority_strategy", "Majority Strategy", "base",
		"Buy/Sell based on majority vote of sub-strategies.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return nil, fmt.Errorf("majority_strategy requires sub-strategies; use via compound strategies")
		})

	add("split_strategy", "Split Strategy", "base",
		"Use one strategy for Buy signals and another for Sell signals.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return nil, fmt.Errorf("split_strategy requires sub-strategies; use via compound strategies")
		})
}

// ─── Trend Strategies ──────────────────────────────────────────────────────

func registerTrend() {
	add("alligator_strategy", "Alligator Strategy", "trend",
		"Buy when price crosses above alligator jaws, sell when crosses below.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewAlligatorStrategy(), nil
		})

	add("apo_strategy", "APO Strategy", "trend",
		"Absolute Price Oscillator strategy. Buy when APO crosses above zero, sell when crosses below.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewApoStrategy(), nil
		})

	add("aroon_strategy", "Aroon Strategy", "trend",
		"Buy when Aroon Up crosses above Aroon Down, sell when Aroon Down crosses above Aroon Up.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewAroonStrategy(), nil
		})

	add("bop_strategy", "BOP Strategy", "trend",
		"Balance of Power strategy. Buy when BOP crosses above zero, sell when crosses below.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewBopStrategy(), nil
		})

	add("cci_strategy", "CCI Strategy", "trend",
		"Commodity Channel Index strategy. Buy when CCI crosses below -100 and back above, sell when crosses above +100 and back below.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewCciStrategy(), nil
		})

	add("cfo_strategy", "CFO Strategy", "trend",
		"Chande Forcast Oscillator strategy.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewCfoStrategy(), nil
		})

	add("dema_strategy", "DEMA Strategy", "trend",
		"Double Exponential Moving Average strategy. Buy when DEMA crosses above price, sell when below.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewDemaStrategy(), nil
		})

	add("envelope_strategy", "Envelope Strategy", "trend",
		"Buy when closing price crosses below the lower envelope band, sell when crosses above the upper band.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewEnvelopeStrategy(), nil
		})

	add("golden_cross_strategy", "Golden Cross Strategy", "trend",
		"Buy when fast EMA crosses above slow EMA (golden cross), sell when fast crosses below slow (death cross).",
		map[string]any{"fast_period": 50, "slow_period": 200},
		func(params map[string]any) (cindicator.Strategy, error) {
			fast := getParamInt(params, "fast_period", 50)
			slow := getParamInt(params, "slow_period", 200)
			return trend.NewGoldenCrossStrategyWith(fast, slow), nil
		})

	add("hma_strategy", "HMA Strategy", "trend",
		"Hull Moving Average strategy. Buy when HMA crosses above price, sell when below.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewHmaStrategy(), nil
		})

	add("kama_strategy", "KAMA Strategy", "trend",
		"Kaufman's Adaptive Moving Average strategy.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewKamaStrategy(), nil
		})

	add("kdj_strategy", "KDJ Strategy", "trend",
		"KDJ indicator strategy. Buy when KDJ crosses below oversold, sell when crosses above overbought.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewKdjStrategy(), nil
		})

	add("macd_strategy", "MACD Strategy", "trend",
		"MACD crossover strategy. Buy when MACD line crosses above signal line, sell when crosses below.",
		map[string]any{"period1": 12, "period2": 26, "period3": 9},
		func(params map[string]any) (cindicator.Strategy, error) {
			p1 := getParamInt(params, "period1", 12)
			p2 := getParamInt(params, "period2", 26)
			p3 := getParamInt(params, "period3", 9)
			return trend.NewMacdStrategyWith(p1, p2, p3), nil
		})

	add("qstick_strategy", "QStick Strategy", "trend",
		"QStick strategy. Buy when QStick crosses above zero, sell when crosses below.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewQstickStrategy(), nil
		})

	add("smma_strategy", "SMMA Strategy", "trend",
		"Smoothed Moving Average strategy. Buy when SMMA crosses above price, sell when below.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewSmmaStrategy(), nil
		})

	add("trima_strategy", "TRIMA Strategy", "trend",
		"Triangular Moving Average strategy. Buy when TRIMA crosses above price, sell when below.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewTrimaStrategy(), nil
		})

	add("triple_moving_average_crossover_strategy", "Triple MA Crossover Strategy", "trend",
		"Triple Moving Average Crossover strategy. Combines three moving averages for signals.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewTripleMovingAverageCrossoverStrategy(), nil
		})

	add("trix_strategy", "TRIX Strategy", "trend",
		"TRIX strategy. Buy when TRIX crosses above zero, sell when crosses below.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewTrixStrategy(), nil
		})

	add("tsi_strategy", "TSI Strategy", "trend",
		"True Strength Index strategy. Buy when TSI crosses above signal line, sell when crosses below.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewTsiStrategy(), nil
		})

	add("vwma_strategy", "VWMA Strategy", "trend",
		"Volume Weighted Moving Average strategy. Buy when VWMA crosses above price, sell when below.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewVwmaStrategy(), nil
		})

	add("weighted_close_strategy", "Weighted Close Strategy", "trend",
		"Weighted Close strategy. Uses (H+L+2*C)/4 as the reference price.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return trend.NewWeightedCloseStrategy(), nil
		})
}

// ─── Momentum Strategies ───────────────────────────────────────────────────

func registerMomentum() {
	add("awesome_oscillator_strategy", "Awesome Oscillator Strategy", "momentum",
		"Buy when Awesome Oscillator crosses above zero, sell when crosses below zero.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return momentum.NewAwesomeOscillatorStrategy(), nil
		})

	add("coppock_curve_strategy", "Coppock Curve Strategy", "momentum",
		"Buy when Coppock Curve crosses above zero, sell when crosses below zero.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return momentum.NewCoppockCurveStrategy(), nil
		})

	add("elder_ray_strategy", "Elder-Ray Strategy", "momentum",
		"Buy when Elder-Ray Bull Power crosses above zero, sell when Bear Power crosses below zero.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return momentum.NewElderRayStrategy(), nil
		})

	add("ichimoku_cloud_strategy", "Ichimoku Cloud Strategy", "momentum",
		"Ichimoku Cloud strategy. Buy when price crosses above the cloud, sell when crosses below.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return momentum.NewIchimokuCloudStrategy(), nil
		})

	add("rsi_strategy", "RSI Strategy", "momentum",
		"Buy when RSI crosses below oversold threshold, sell when RSI crosses above overbought threshold.",
		map[string]any{"buy_at": 30.0, "sell_at": 70.0},
		func(params map[string]any) (cindicator.Strategy, error) {
			buyAt := getParamFloat(params, "buy_at", 30)
			sellAt := getParamFloat(params, "sell_at", 70)
			return momentum.NewRsiStrategyWith(buyAt, sellAt), nil
		})

	add("stochastic_oscillator_strategy", "Stochastic Oscillator Strategy", "momentum",
		"Buy when %K crosses below oversold threshold, sell when %K crosses above overbought threshold.",
		map[string]any{"buy_at": 20.0, "sell_at": 80.0},
		func(params map[string]any) (cindicator.Strategy, error) {
			buyAt := getParamFloat(params, "buy_at", 20)
			sellAt := getParamFloat(params, "sell_at", 80)
			return momentum.NewStochasticOscillatorStrategyWith(buyAt, sellAt), nil
		})

	add("stochastic_rsi_strategy", "Stochastic RSI Strategy", "momentum",
		"Stochastic RSI strategy. Combines RSI with Stochastic for more sensitive signals.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return momentum.NewStochasticRsiStrategy(), nil
		})

	add("triple_rsi_strategy", "Triple RSI Strategy", "momentum",
		"Triple RSI strategy. Uses three RSI periods for signal confirmation.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return momentum.NewTripleRsiStrategy(), nil
		})

	add("williams_r_strategy", "Williams %R Strategy", "momentum",
		"Buy when Williams %R crosses below oversold, sell when crosses above overbought.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return momentum.NewWilliamsRStrategy(), nil
		})
}

// ─── Volatility Strategies ─────────────────────────────────────────────────

func registerVolatility() {
	add("bollinger_bands_strategy", "Bollinger Bands Strategy", "volatility",
		"Buy when price touches lower band, sell when price touches upper band.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return volatility.NewBollingerBandsStrategy(), nil
		})

	add("donchian_channel_breakout_strategy", "Donchian Channel Breakout Strategy", "volatility",
		"Buy when price breaks above the Donchian channel high, sell when breaks below the low.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return volatility.NewDonchianChannelBreakoutStrategy(), nil
		})

	add("keltner_channel_strategy", "Keltner Channel Strategy", "volatility",
		"Buy when price touches lower Keltner channel band, sell when touches upper band.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return volatility.NewKeltnerChannelStrategy(), nil
		})

	add("super_trend_strategy", "Super Trend Strategy", "volatility",
		"Buy when price crosses above Super Trend line, sell when crosses below.",
		map[string]any{"period": 10, "multiplier": 3.0},
		func(params map[string]any) (cindicator.Strategy, error) {
			period := getParamInt(params, "period", 10)
			multiplier := getParamFloat(params, "multiplier", 3)
			return volatility.NewSuperTrendStrategyWith(
				cvolatility.NewSuperTrendWithPeriod[float64](period, multiplier),
			), nil
		})
}

// ─── Volume Strategies ─────────────────────────────────────────────────────

func registerVolume() {
	add("chaikin_money_flow_strategy", "Chaikin Money Flow Strategy", "volume",
		"Buy when Chaikin Money Flow crosses above zero, sell when crosses below zero.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return volume.NewChaikinMoneyFlowStrategy(), nil
		})

	add("ease_of_movement_strategy", "Ease of Movement Strategy", "volume",
		"Buy when Ease of Movement crosses above zero, sell when crosses below zero.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return volume.NewEaseOfMovementStrategy(), nil
		})

	add("force_index_strategy", "Force Index Strategy", "volume",
		"Buy when Force Index crosses above zero, sell when crosses below zero.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return volume.NewForceIndexStrategy(), nil
		})

	add("money_flow_index_strategy", "Money Flow Index Strategy", "volume",
		"Buy when MFI crosses below oversold threshold, sell when crosses above overbought threshold.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return volume.NewMoneyFlowIndexStrategy(), nil
		})

	add("negative_volume_index_strategy", "Negative Volume Index Strategy", "volume",
		"Buy when NVI crosses above its moving average, sell when crosses below.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return volume.NewNegativeVolumeIndexStrategy(), nil
		})

	add("obv_strategy", "OBV Strategy", "volume",
		"Buy when On-Balance Volume crosses above its SMA, sell when crosses below.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return volume.NewObvStrategy(), nil
		})

	add("weighted_average_price_strategy", "Weighted Average Price Strategy", "volume",
		"Weighted Average Price strategy. Uses VWAP as the reference for signals.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return volume.NewWeightedAveragePriceStrategy(), nil
		})
}

// ─── Compound Strategies ───────────────────────────────────────────────────

func registerCompound() {
	add("macd_rsi_strategy", "MACD + RSI Strategy", "compound",
		"Combines MACD and RSI strategies. Requires both to agree before generating signals.",
		map[string]any{"rsi_buy_at": 30.0, "rsi_sell_at": 70.0},
		func(params map[string]any) (cindicator.Strategy, error) {
			buyAt := getParamFloat(params, "rsi_buy_at", 30)
			sellAt := getParamFloat(params, "rsi_sell_at", 70)
			return compound.NewMacdRsiStrategyWith(buyAt, sellAt), nil
		})
}

// ─── Decorator Strategies ──────────────────────────────────────────────────

func registerDecorator() {
	add("inverse_strategy", "Inverse Strategy", "decorator",
		"Inverts signals from an inner strategy. Buy becomes Sell and vice versa.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return nil, fmt.Errorf("inverse_strategy requires an inner strategy; wrap a standalone strategy")
		})

	add("no_loss_strategy", "No Loss Strategy", "decorator",
		"Prevents selling at a loss. Only sells if price is above the purchase price.",
		nil,
		func(params map[string]any) (cindicator.Strategy, error) {
			return nil, fmt.Errorf("no_loss_strategy requires an inner strategy; wrap a standalone strategy")
		})

	add("stop_loss_strategy", "Stop Loss Strategy", "decorator",
		"Applies a stop-loss to an inner strategy. Sells if loss exceeds the threshold.",
		map[string]any{"loss_percentage": 0.1},
		func(params map[string]any) (cindicator.Strategy, error) {
			return nil, fmt.Errorf("stop_loss_strategy requires an inner strategy; wrap a standalone strategy")
		})
}

// ─── Helpers ───────────────────────────────────────────────────────────────

func add(typeKey, name, category, description string, defaultParams map[string]any, constructor func(map[string]any) (cindicator.Strategy, error)) {
	Catalog[typeKey] = StrategyDef{
		Type:          typeKey,
		Name:          name,
		Category:      category,
		Description:   description,
		DefaultParams: defaultParams,
		Constructor:   constructor,
	}
}

func getParamInt(params map[string]any, key string, defaultVal int) int {
	if params == nil {
		return defaultVal
	}
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return defaultVal
	}
}

func getParamFloat(params map[string]any, key string, defaultVal float64) float64 {
	if params == nil {
		return defaultVal
	}
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return defaultVal
	}
}

func getParamString(params map[string]any, key string, defaultVal string) string {
	if params == nil {
		return defaultVal
	}
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	s, ok := v.(string)
	if !ok {
		return defaultVal
	}
	return s
}
