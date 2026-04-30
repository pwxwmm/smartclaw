package costguard

import (
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"sort"
	"time"
)

// CostForecast represents a full cost prediction with trend analysis.
type CostForecast struct {
	TodayRemaining    float64         `json:"today_remaining"`
	WeekProjected     float64         `json:"week_projected"`
	MonthProjected    float64         `json:"month_projected"`
	TrendDirection    string          `json:"trend_direction"`    // "increasing", "stable", "decreasing"
	TrendSlope        float64         `json:"trend_slope"`
	BudgetSustainDays int             `json:"budget_sustain_days"` // days until budget exhaustion
	DailyForecasts    []DailyForecast `json:"daily_forecasts"`     // 7-day forecast
	RiskLevel         string          `json:"risk_level"`          // "low", "moderate", "high", "critical"
	Recommendations   []ForecastRec   `json:"recommendations"`
}

// DailyForecast is a single day's predicted cost.
type DailyForecast struct {
	Date          string  `json:"date"`
	PredictedCost float64 `json:"predicted_cost"`
	Confidence    float64 `json:"confidence"`    // 0-1
	LowerBound    float64 `json:"lower_bound"`
	UpperBound    float64 `json:"upper_bound"`
}

// ForecastRec is a forecast-based recommendation.
type ForecastRec struct {
	Type        string  `json:"type"`        // "budget_adjust", "model_switch", "usage_pattern"
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Impact      float64 `json:"impact"`      // estimated $ impact
	Priority    string  `json:"priority"`    // "high", "medium", "low"
}

// ProjectCost represents cost aggregation per project/directory.
type ProjectCost struct {
	ProjectName    string              `json:"project_name"`
	TotalCost      float64             `json:"total_cost"`
	SessionCount   int                 `json:"session_count"`
	AvgPerSession  float64             `json:"avg_per_session"`
	ModelBreakdown []ModelCostBreakdown `json:"model_breakdown"`
}

// TaskCostEstimate is a pre-task cost estimate.
type TaskCostEstimate struct {
	TaskType            string  `json:"task_type"`
	EstimatedCost       float64 `json:"estimated_cost"`
	Confidence          float64 `json:"confidence"`
	CheaperAlt          string  `json:"cheaper_alternative,omitempty"`
	CheaperCost         float64 `json:"cheaper_alternative_cost,omitempty"`
}

// WeeklyReport is a structured weekly cost summary.
type WeeklyReport struct {
	Period          string             `json:"period"`
	TotalCost       float64            `json:"total_cost"`
	VsPreviousWeek  float64            `json:"vs_previous_week"` // percentage change
	TopModel        ModelCostBreakdown `json:"top_model"`
	TopProject      ProjectCost        `json:"top_project"`
	Forecast        CostForecast       `json:"forecast"`
	SavingsRealized float64            `json:"savings_realized"` // from auto-downgrade
	Tips            []OptimizationTip  `json:"tips"`
}

// dailyCostPoint is an internal type for daily aggregated cost.
type dailyCostPoint struct {
	Date string
	Cost float64
}

// taskTypeProfile defines average token usage per task type.
type taskTypeProfile struct {
	AvgInputTokens  int
	AvgOutputTokens int
}

var taskTypeProfiles = map[string]taskTypeProfile{
	"code_generation": {AvgInputTokens: 2000, AvgOutputTokens: 1500},
	"code_review":    {AvgInputTokens: 3000, AvgOutputTokens: 800},
	"debugging":      {AvgInputTokens: 2500, AvgOutputTokens: 1200},
	"explanation":    {AvgInputTokens: 1000, AvgOutputTokens: 600},
	"refactoring":    {AvgInputTokens: 3000, AvgOutputTokens: 2000},
	"testing":        {AvgInputTokens: 2000, AvgOutputTokens: 1500},
	"deployment":     {AvgInputTokens: 800, AvgOutputTokens: 400},
}

// ForecastCost produces a full cost forecast using linear regression on
// historical daily costs from the cost_snapshots table.
func ForecastCost(db *sql.DB, budget BudgetConfig) (*CostForecast, error) {
	if db == nil {
		return emptyForecast(budget), nil
	}

	// Load last 30 days aggregated by day.
	points, err := loadDailyCosts(db, 30)
	if err != nil {
		return nil, fmt.Errorf("forecast: load daily costs: %w", err)
	}

	if len(points) < 2 {
		return emptyForecast(budget), nil
	}

	// Fit linear regression: cost = a + b*dayIndex
	n := float64(len(points))
	var sumX, sumY, sumXY, sumX2 float64
	for i, p := range points {
		x := float64(i)
		y := p.Cost
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	denom := n*sumX2 - sumX*sumX
	if math.Abs(denom) < 1e-10 {
		// All same x — degenerate case
		return emptyForecast(budget), nil
	}

	slope := (n*sumXY - sumX*sumY) / denom
	intercept := (sumY - slope*sumX) / n

	// Trend direction.
	trendDirection := "stable"
	if slope > 0.05 {
		trendDirection = "increasing"
	} else if slope < -0.05 {
		trendDirection = "decreasing"
	}

	// Compute daily average from historical data.
	var totalCost float64
	for _, p := range points {
		totalCost += p.Cost
	}
	dailyAvg := totalCost / n

	// Today remaining.
	todayStr := time.Now().Format("2006-01-02")
	var todayCost float64
	db.QueryRow(`SELECT COALESCE(SUM(cost_usd), 0) FROM cost_snapshots WHERE date = ?`, todayStr).Scan(&todayCost)
	todayRemaining := budget.DailyLimitUSD - todayCost
	if todayRemaining < 0 {
		todayRemaining = 0
	}

	// 7-day forecast.
	forecasts := make([]DailyForecast, 7)
	for i := 0; i < 7; i++ {
		dayIndex := float64(len(points) + i)
		predicted := intercept + slope*dayIndex
		if predicted < 0 {
			predicted = 0
		}

		// Confidence: more data points = higher confidence, capped at 0.9.
		confidence := math.Min(0.4+0.05*float64(len(points)), 0.9)

		// Confidence band: wider for further predictions.
		bandWidth := dailyAvg * 0.3 * (1 + float64(i)*0.1)
		if bandWidth < 0.01 {
			bandWidth = 0.01
		}

		date := time.Now().AddDate(0, 0, i+1).Format("2006-01-02")
		forecasts[i] = DailyForecast{
			Date:          date,
			PredictedCost: roundTo(predicted, 4),
			Confidence:    roundTo(confidence, 2),
			LowerBound:    roundTo(math.Max(0, predicted-bandWidth), 4),
			UpperBound:    roundTo(predicted+bandWidth, 4),
		}
	}

	// Week and month projected.
	var weekProjected, monthProjected float64
	// Week: sum of next 7 days
	for _, f := range forecasts {
		weekProjected += f.PredictedCost
	}
	// Month: dailyAvg * 30 (or use regression)
	monthProjected = dailyAvg * 30

	// Budget sustain days.
	budgetSustainDays := 0
	if dailyAvg > 0 && budget.DailyLimitUSD > 0 {
		budgetSustainDays = int(budget.DailyLimitUSD / dailyAvg)
	} else if budget.DailyLimitUSD > 0 {
		budgetSustainDays = 999 // essentially infinite
	}

	// Risk level based on budget_sustain_days.
	riskLevel := "low"
	if budget.DailyLimitUSD <= 0 {
		riskLevel = "low"
	} else if budgetSustainDays < 7 {
		riskLevel = "critical"
	} else if budgetSustainDays < 14 {
		riskLevel = "high"
	} else if budgetSustainDays < 30 {
		riskLevel = "moderate"
	}

	// Generate recommendations.
	recs := generateForecastRecommendations(budget, trendDirection, slope, dailyAvg, budgetSustainDays, weekProjected)

	return &CostForecast{
		TodayRemaining:    roundTo(todayRemaining, 4),
		WeekProjected:     roundTo(weekProjected, 4),
		MonthProjected:    roundTo(monthProjected, 4),
		TrendDirection:    trendDirection,
		TrendSlope:        roundTo(slope, 4),
		BudgetSustainDays: budgetSustainDays,
		DailyForecasts:    forecasts,
		RiskLevel:         riskLevel,
		Recommendations:   recs,
	}, nil
}

// EstimateTaskCost returns a cost estimate for a given task type based on
// historical data and model pricing heuristics.
func EstimateTaskCost(db *sql.DB, taskType string) (*TaskCostEstimate, error) {
	profile, ok := taskTypeProfiles[taskType]
	if !ok {
		// Default profile for unknown task types.
		profile = taskTypeProfile{AvgInputTokens: 1500, AvgOutputTokens: 800}
	}

	// Use a default model pricing (Sonnet) for estimation.
	defaultTier := PricingTier{
		InputPricePer1M:  3.0,
		OutputPricePer1M: 15.0,
	}

	inputCost := float64(profile.AvgInputTokens) * defaultTier.InputPricePer1M / 1_000_000
	outputCost := float64(profile.AvgOutputTokens) * defaultTier.OutputPricePer1M / 1_000_000
	estimatedCost := inputCost + outputCost

	// Try to refine with historical data if available.
	confidence := 0.5
	if db != nil {
		var avgCost float64
		var count int
		err := db.QueryRow(`
			SELECT COALESCE(AVG(cost_usd), 0), COUNT(*)
			FROM cost_snapshots
			WHERE date >= date('now', '-30 days')
		`).Scan(&avgCost, &count)
		if err == nil && count > 10 {
			// Blend historical average with heuristic estimate.
			estimatedCost = estimatedCost*0.4 + avgCost*0.6
			confidence = math.Min(0.5+0.05*float64(count), 0.85)
		}
	}

	// Compute cheaper alternative using Haiku pricing.
	haikuTier := PricingTier{
		InputPricePer1M:  0.8,
		OutputPricePer1M: 4.0,
	}
	haikuInputCost := float64(profile.AvgInputTokens) * haikuTier.InputPricePer1M / 1_000_000
	haikuOutputCost := float64(profile.AvgOutputTokens) * haikuTier.OutputPricePer1M / 1_000_000
	haikuCost := haikuInputCost + haikuOutputCost

	estimate := &TaskCostEstimate{
		TaskType:       taskType,
		EstimatedCost:  roundTo(estimatedCost, 4),
		Confidence:     roundTo(confidence, 2),
		CheaperAlt:     "claude-3-5-haiku-20241022",
		CheaperCost:    roundTo(haikuCost, 4),
	}

	return estimate, nil
}

// GetProjectCosts aggregates cost per project/directory over the given number of days.
// It uses session metadata to group costs by project.
func GetProjectCosts(db *sql.DB, days int) ([]ProjectCost, error) {
	if db == nil {
		return []ProjectCost{}, nil
	}

	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	// Group sessions by title prefix as a proxy for project.
	// Sessions that share the same first word or have a common pattern are grouped.
	rows, err := db.Query(`
		SELECT
			COALESCE(NULLIF(SUBSTR(title, 1, INSTR(title || ' ', ' ') - 1), ''), 'untitled') as project,
			COUNT(*) as session_count,
			COALESCE(SUM(cost), 0) as total_cost
		FROM sessions
		WHERE created_at >= ? AND cost > 0
		GROUP BY project
		ORDER BY total_cost DESC
	`, since)
	if err != nil {
		return nil, fmt.Errorf("project costs query: %w", err)
	}
	defer rows.Close()

	var projects []ProjectCost
	for rows.Next() {
		var p ProjectCost
		if err := rows.Scan(&p.ProjectName, &p.SessionCount, &p.TotalCost); err != nil {
			continue
		}
		if p.SessionCount > 0 {
			p.AvgPerSession = roundTo(p.TotalCost/float64(p.SessionCount), 4)
		}

		// Get model breakdown for this project.
		breakdown, err := getProjectModelBreakdown(db, p.ProjectName, since)
		if err != nil {
			slog.Warn("cost forecast: failed to get model breakdown", "project", p.ProjectName, "error", err)
		}
		p.ModelBreakdown = breakdown
		projects = append(projects, p)
	}

	if projects == nil {
		projects = []ProjectCost{}
	}
	return projects, nil
}

// GenerateWeeklyReport creates a structured weekly cost report.
func GenerateWeeklyReport(db *sql.DB, budget BudgetConfig) (*WeeklyReport, error) {
	now := time.Now()
	weekStart := now.AddDate(0, 0, -7).Format("2006-01-02")
	prevWeekStart := now.AddDate(0, 0, -14).Format("2006-01-02")
	period := fmt.Sprintf("%s to %s", weekStart, now.Format("2006-01-02"))

	var weekCost, prevWeekCost float64
	if db != nil {
		db.QueryRow(`SELECT COALESCE(SUM(cost_usd), 0) FROM cost_snapshots WHERE date >= ?`, weekStart).Scan(&weekCost)
		db.QueryRow(`SELECT COALESCE(SUM(cost_usd), 0) FROM cost_snapshots WHERE date >= ? AND date < ?`, prevWeekStart, weekStart).Scan(&prevWeekCost)
	}

	vsPrevious := 0.0
	if prevWeekCost > 0 {
		vsPrevious = ((weekCost - prevWeekCost) / prevWeekCost) * 100
	}

	// Top model.
	breakdown, _ := GetModelBreakdown(db, 7)
	var topModel ModelCostBreakdown
	if len(breakdown) > 0 {
		topModel = breakdown[0]
	}

	// Top project.
	projects, _ := GetProjectCosts(db, 7)
	var topProject ProjectCost
	if len(projects) > 0 {
		topProject = projects[0]
	}

	// Forecast.
	forecast, _ := ForecastCost(db, budget)
	if forecast == nil {
		forecast = emptyForecast(budget)
	}

	// Estimate savings from auto-downgrade (heuristic: 10-30% of cost that was on default models).
	savingsRealized := 0.0
	if db != nil {
		var downgradedCost float64
		db.QueryRow(`
			SELECT COALESCE(SUM(cost_usd), 0) FROM cost_snapshots
			WHERE date >= ? AND model LIKE '%haiku%' OR model LIKE '%mini%' OR model LIKE '%flash%'
		`, weekStart).Scan(&downgradedCost)
		// If there were queries on cheaper models, estimate savings as the cost difference
		// vs using the default (Sonnet) model — roughly 3x savings.
		savingsRealized = downgradedCost * 2.0
	}

	// Tips.
	tips := GenerateOptimizationTips(breakdown, budget)

	return &WeeklyReport{
		Period:          period,
		TotalCost:       roundTo(weekCost, 4),
		VsPreviousWeek:  roundTo(vsPrevious, 1),
		TopModel:        topModel,
		TopProject:      topProject,
		Forecast:        *forecast,
		SavingsRealized: roundTo(savingsRealized, 4),
		Tips:            tips,
	}, nil
}

// loadDailyCosts aggregates cost_snapshots by day for the last N days.
func loadDailyCosts(db *sql.DB, days int) ([]dailyCostPoint, error) {
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	rows, err := db.Query(`
		SELECT date, SUM(cost_usd) as total_cost
		FROM cost_snapshots
		WHERE date >= ?
		GROUP BY date
		ORDER BY date ASC
	`, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []dailyCostPoint
	for rows.Next() {
		var p dailyCostPoint
		if err := rows.Scan(&p.Date, &p.Cost); err != nil {
			continue
		}
		points = append(points, p)
	}
	return points, nil
}

// getProjectModelBreakdown returns model cost breakdown for a specific project.
func getProjectModelBreakdown(db *sql.DB, projectPrefix string, since string) ([]ModelCostBreakdown, error) {
	likePattern := projectPrefix + "%"
	rows, err := db.Query(`
		SELECT cs.model,
			SUM(cs.cost_usd) as total_cost,
			SUM(cs.query_count) as total_queries
		FROM cost_snapshots cs
		JOIN sessions s ON s.model = cs.model AND s.created_at >= ?
		WHERE cs.date >= ? AND s.title LIKE ?
		GROUP BY cs.model
		ORDER BY total_cost DESC
	`, since, since, likePattern)
	if err != nil {
		return []ModelCostBreakdown{}, nil
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

// generateForecastRecommendations creates recommendations based on forecast data.
func generateForecastRecommendations(budget BudgetConfig, trend string, slope, dailyAvg float64, sustainDays int, weekProjected float64) []ForecastRec {
	var recs []ForecastRec

	if trend == "increasing" && budget.DailyLimitUSD > 0 {
		recs = append(recs, ForecastRec{
			Type:        "usage_pattern",
			Title:       "Cost trend is increasing",
			Description: fmt.Sprintf("Daily costs are rising by ~$%.4f/day. Consider reviewing recent usage patterns.", slope),
			Impact:      slope * 30,
			Priority:    "high",
		})
	}

	if sustainDays > 0 && sustainDays < 14 && budget.DailyLimitUSD > 0 {
		recs = append(recs, ForecastRec{
			Type:        "budget_adjust",
			Title:       "Budget may be exhausted soon",
			Description: fmt.Sprintf("At current spend rate ($%.2f/day), your budget will last ~%d more days. Consider increasing your daily limit or reducing usage.", dailyAvg, sustainDays),
			Impact:      dailyAvg * float64(sustainDays),
			Priority:    "high",
		})
	}

	if weekProjected > budget.DailyLimitUSD*5 && budget.DailyLimitUSD > 0 {
		recs = append(recs, ForecastRec{
			Type:        "model_switch",
			Title:       "Switch to cheaper models for routine tasks",
			Description: fmt.Sprintf("Projected weekly cost is $%.2f, which exceeds 5x your daily budget. Using Haiku for simple queries could save up to 70%%.", weekProjected),
			Impact:      weekProjected * 0.5,
			Priority:    "medium",
		})
	}

	if trend == "stable" && dailyAvg > 0 {
		recs = append(recs, ForecastRec{
			Type:        "usage_pattern",
			Title:       "Spending is stable",
			Description: fmt.Sprintf("Your daily average is $%.2f with a stable trend. Consider prompt caching to further reduce costs.", dailyAvg),
			Impact:      dailyAvg * 0.1 * 30,
			Priority:    "low",
		})
	}

	if trend == "decreasing" {
		recs = append(recs, ForecastRec{
			Type:        "usage_pattern",
			Title:       "Costs are decreasing",
			Description: "Your spending trend is downward. Keep up the efficient usage patterns.",
			Impact:      math.Abs(slope) * 30,
			Priority:    "low",
		})
	}

	sort.Slice(recs, func(i, j int) bool {
		// Sort by priority: high > medium > low
		prioOrder := map[string]int{"high": 3, "medium": 2, "low": 1}
		return prioOrder[recs[i].Priority] > prioOrder[recs[j].Priority]
	})

	if recs == nil {
		recs = []ForecastRec{}
	}
	return recs
}

// emptyForecast returns a zero-valued forecast for when there's insufficient data.
func emptyForecast(budget BudgetConfig) *CostForecast {
	forecasts := make([]DailyForecast, 7)
	for i := 0; i < 7; i++ {
		date := time.Now().AddDate(0, 0, i+1).Format("2006-01-02")
		forecasts[i] = DailyForecast{
			Date:          date,
			PredictedCost: 0,
			Confidence:    0.1,
			LowerBound:    0,
			UpperBound:    0,
		}
	}

	return &CostForecast{
		TodayRemaining:    budget.DailyLimitUSD,
		WeekProjected:     0,
		MonthProjected:    0,
		TrendDirection:    "stable",
		TrendSlope:        0,
		BudgetSustainDays: 0,
		DailyForecasts:    forecasts,
		RiskLevel:         "low",
		Recommendations:   []ForecastRec{},
	}
}

// roundTo rounds a float64 to the given number of decimal places.
func roundTo(v float64, places int) float64 {
	pow := math.Pow10(places)
	return math.Round(v*pow) / pow
}
