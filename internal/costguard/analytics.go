package costguard

import (
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"time"
)

// CostHistoryEntry represents a single day's cost for a model.
type CostHistoryEntry struct {
	Date         string  `json:"date"`
	Model        string  `json:"model"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	CostUSD      float64 `json:"cost_usd"`
	QueryCount   int     `json:"query_count"`
}

// ModelCostBreakdown represents cost aggregation per model.
type ModelCostBreakdown struct {
	Model          string  `json:"model"`
	TotalCost      float64 `json:"total_cost"`
	QueryCount     int     `json:"query_count"`
	AvgCostPerQuery float64 `json:"avg_cost_per_query"`
	Percentage     float64 `json:"percentage"`
}

// OptimizationTip represents a cost optimization suggestion.
type OptimizationTip struct {
	Type        string  `json:"type"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	SavingsUSD  float64 `json:"savings_usd"`
}

// CostDashboard is the full analytics dashboard response.
type CostDashboard struct {
	CurrentSession   CostSnapshot       `json:"current_session"`
	TodayTotal       float64            `json:"today_total"`
	WeekTotal        float64            `json:"week_total"`
	MonthTotal       float64            `json:"month_total"`
	BudgetRemaining  float64            `json:"budget_remaining"`
	BudgetFraction   float64            `json:"budget_fraction"`
	History          []CostHistoryEntry `json:"history"`
	ModelBreakdown   []ModelCostBreakdown `json:"model_breakdown"`
	OptimizationTips []OptimizationTip  `json:"optimization_tips"`
	DailyAverages    float64            `json:"daily_averages"`
	ProjectedMonthly float64            `json:"projected_monthly"`
}

// RecordCostSnapshot upserts a daily cost snapshot for a model.
func RecordCostSnapshot(db *sql.DB, model string, inputTokens, outputTokens, cacheRead, cacheCreate int64, costUSD float64) {
	if db == nil {
		return
	}
	today := time.Now().Format("2006-01-02")

	_, err := db.Exec(`
		INSERT INTO cost_snapshots (date, model, input_tokens, output_tokens, cache_read_tokens, cache_create_tokens, cost_usd, query_count)
		VALUES (?, ?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(date, model) DO UPDATE SET
			input_tokens = input_tokens + excluded.input_tokens,
			output_tokens = output_tokens + excluded.output_tokens,
			cache_read_tokens = cache_read_tokens + excluded.cache_read_tokens,
			cache_create_tokens = cache_create_tokens + excluded.cache_create_tokens,
			cost_usd = cost_usd + excluded.cost_usd,
			query_count = query_count + 1
	`, today, model, inputTokens, outputTokens, cacheRead, cacheCreate, costUSD)

	if err != nil {
		slog.Warn("cost analytics: failed to record snapshot", "error", err)
	}
}

// GetCostHistory returns daily cost history for the last N days.
func GetCostHistory(db *sql.DB, days int) ([]CostHistoryEntry, error) {
	if db == nil {
		return []CostHistoryEntry{}, nil
	}

	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	rows, err := db.Query(`
		SELECT date, model, input_tokens, output_tokens, cost_usd, query_count
		FROM cost_snapshots
		WHERE date >= ?
		ORDER BY date ASC, model ASC
	`, since)
	if err != nil {
		return nil, fmt.Errorf("cost history query: %w", err)
	}
	defer rows.Close()

	var entries []CostHistoryEntry
	for rows.Next() {
		var e CostHistoryEntry
		if err := rows.Scan(&e.Date, &e.Model, &e.InputTokens, &e.OutputTokens, &e.CostUSD, &e.QueryCount); err != nil {
			continue
		}
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []CostHistoryEntry{}
	}
	return entries, nil
}

// GetModelBreakdown returns cost per model for the last N days.
func GetModelBreakdown(db *sql.DB, days int) ([]ModelCostBreakdown, error) {
	if db == nil {
		return []ModelCostBreakdown{}, nil
	}

	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	rows, err := db.Query(`
		SELECT model,
			SUM(cost_usd) as total_cost,
			SUM(query_count) as total_queries
		FROM cost_snapshots
		WHERE date >= ?
		GROUP BY model
		ORDER BY total_cost DESC
	`, since)
	if err != nil {
		return nil, fmt.Errorf("model breakdown query: %w", err)
	}
	defer rows.Close()

	var breakdown []ModelCostBreakdown
	var totalCost float64
	for rows.Next() {
		var b ModelCostBreakdown
		if err := rows.Scan(&b.Model, &b.TotalCost, &b.QueryCount); err != nil {
			continue
		}
		if b.QueryCount > 0 {
			b.AvgCostPerQuery = b.TotalCost / float64(b.QueryCount)
		}
		totalCost += b.TotalCost
		breakdown = append(breakdown, b)
	}

	for i := range breakdown {
		if totalCost > 0 {
			breakdown[i].Percentage = (breakdown[i].TotalCost / totalCost) * 100
		}
	}

	if breakdown == nil {
		breakdown = []ModelCostBreakdown{}
	}
	return breakdown, nil
}

// GetDashboard returns the full cost dashboard.
func GetDashboard(guard *CostGuard, db *sql.DB) (*CostDashboard, error) {
	snapshot := guard.Snapshot("")

	today := time.Now().Format("2006-01-02")
	weekAgo := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	monthAgo := time.Now().AddDate(0, 0, -30).Format("2006-01-02")

	var todayTotal, weekTotal, monthTotal float64

	if db != nil {
		db.QueryRow(`SELECT COALESCE(SUM(cost_usd), 0) FROM cost_snapshots WHERE date = ?`, today).Scan(&todayTotal)
		db.QueryRow(`SELECT COALESCE(SUM(cost_usd), 0) FROM cost_snapshots WHERE date >= ?`, weekAgo).Scan(&weekTotal)
		db.QueryRow(`SELECT COALESCE(SUM(cost_usd), 0) FROM cost_snapshots WHERE date >= ?`, monthAgo).Scan(&monthTotal)
	}

	history, _ := GetCostHistory(db, 30)
	breakdown, _ := GetModelBreakdown(db, 30)

	config := guard.GetConfig()
	budgetRemaining := config.DailyLimitUSD - todayTotal
	budgetFraction := 0.0
	if config.DailyLimitUSD > 0 {
		budgetFraction = todayTotal / config.DailyLimitUSD
	}
	if budgetRemaining < 0 {
		budgetRemaining = 0
	}
	if budgetFraction > 1.0 {
		budgetFraction = 1.0
	}

	dailyAvg := 0.0
	if len(history) > 0 {
		var total float64
		daySet := make(map[string]bool)
		for _, h := range history {
			total += h.CostUSD
			daySet[h.Date] = true
		}
		if len(daySet) > 0 {
			dailyAvg = total / float64(len(daySet))
		}
	}

	projectedMonthly := dailyAvg * 30

	tips := GenerateOptimizationTips(breakdown, config)

	return &CostDashboard{
		CurrentSession:   snapshot,
		TodayTotal:       todayTotal,
		WeekTotal:        weekTotal,
		MonthTotal:       monthTotal,
		BudgetRemaining:  budgetRemaining,
		BudgetFraction:   budgetFraction,
		History:          history,
		ModelBreakdown:   breakdown,
		OptimizationTips: tips,
		DailyAverages:    dailyAvg,
		ProjectedMonthly: projectedMonthly,
	}, nil
}

// GenerateOptimizationTips produces actionable cost-saving suggestions.
func GenerateOptimizationTips(breakdown []ModelCostBreakdown, budget BudgetConfig) []OptimizationTip {
	var tips []OptimizationTip

	modelTiers := map[string]string{
		"claude-opus-4-20250514": "heavy",
		"claude-sonnet-4-20250514": "default",
		"claude-sonnet-4-5":       "default",
		"claude-3-5-sonnet-20241022": "default",
		"claude-3-5-haiku-20241022": "fast",
		"gpt-4o":       "default",
		"gpt-4o-mini":  "fast",
		"gemini-2.5-pro":  "default",
		"gemini-2.5-flash": "fast",
		"glm-4-plus": "default",
	}

	haikuInputPer1M := 0.8
	haikuOutputPer1M := 4.0
	sonnetInputPer1M := 3.0
	sonnetOutputPer1M := 15.0

	for _, b := range breakdown {
		tier := modelTiers[b.Model]
		if tier == "heavy" {
			savings := b.TotalCost * 0.8
			monthlySavings := savings * 4
			tips = append(tips, OptimizationTip{
				Type:        "model_switch",
				Title:       "Switch from " + shortModelName(b.Model) + " to Haiku",
				Description: fmt.Sprintf("Your queries with %s cost $%.2f. Using Haiku for non-critical queries could save ~$%.2f/month.", shortModelName(b.Model), b.TotalCost, monthlySavings),
				SavingsUSD:  monthlySavings,
			})
		}
	}

	if budget.DailyLimitUSD > 0 {
		totalCost := 0.0
		for _, b := range breakdown {
			totalCost += b.TotalCost
		}
		if totalCost > budget.DailyLimitUSD*0.7 {
			tips = append(tips, OptimizationTip{
				Type:        "budget_alert",
				Title:       "Budget usage is high",
				Description: fmt.Sprintf("You've spent $%.2f of your $%.2f daily budget (%.0f%%). Consider using /fast for non-critical queries.", totalCost, budget.DailyLimitUSD, (totalCost/budget.DailyLimitUSD)*100),
				SavingsUSD:  0,
			})
		}
	}

	if len(breakdown) > 0 {
		totalCost := 0.0
		for _, b := range breakdown {
			totalCost += b.TotalCost
		}
		if totalCost > 5.0 {
			potentialSavings := totalCost * 0.1
			tips = append(tips, OptimizationTip{
				Type:        "cache_usage",
				Title:       "Enable prompt caching",
				Description: fmt.Sprintf("Cache token usage appears low. Enabling prompt caching for repeated contexts could save ~$%.2f/month.", potentialSavings*30),
				SavingsUSD:  potentialSavings * 30,
			})
		}

		for _, b := range breakdown {
			tier := modelTiers[b.Model]
			if tier == "default" && b.QueryCount > 20 {
				savingsPerQuery := (sonnetInputPer1M + sonnetOutputPer1M)/2 - (haikuInputPer1M + haikuOutputPer1M)/2
				monthlySavings := float64(b.QueryCount) * savingsPerQuery * 0.3 * 30 / 1_000_000 * 1000
				if monthlySavings > 0.5 {
					tips = append(tips, OptimizationTip{
						Type:        "model_switch",
						Title:       "Use Haiku for simple queries",
						Description: fmt.Sprintf("You have %d queries with %s. Routing simple queries to Haiku could save ~$%.2f/month.", b.QueryCount, shortModelName(b.Model), monthlySavings),
						SavingsUSD:  monthlySavings,
					})
				}
			}
		}
	}

	sort.Slice(tips, func(i, j int) bool {
		return tips[i].SavingsUSD > tips[j].SavingsUSD
	})

	if tips == nil {
		tips = []OptimizationTip{}
	}
	return tips
}

func shortModelName(model string) string {
	shortNames := map[string]string{
		"claude-opus-4-20250514":      "Opus 4",
		"claude-sonnet-4-20250514":    "Sonnet 4",
		"claude-sonnet-4-5":           "Sonnet 4.5",
		"claude-3-5-sonnet-20241022":  "Sonnet 3.5",
		"claude-3-5-haiku-20241022":   "Haiku 3.5",
		"gpt-4o":                      "GPT-4o",
		"gpt-4o-mini":                 "GPT-4o Mini",
		"gemini-2.5-pro":              "Gemini 2.5 Pro",
		"gemini-2.5-flash":            "Gemini 2.5 Flash",
		"glm-4-plus":                  "GLM-4 Plus",
		"sre-model":                   "SRE Model",
	}
	if name, ok := shortNames[model]; ok {
		return name
	}
	return model
}
