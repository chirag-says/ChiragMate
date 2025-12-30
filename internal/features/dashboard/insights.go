package dashboard

import (
	"fmt"
	"sort"

	"github.com/budgetmate/web/internal/database"
)

// Insight represents a smart nudge for the user
type Insight struct {
	Message    string
	Category   string  // The category being highlighted (if any)
	Percentage float64 // The percentage for context
	IsPositive bool    // Whether this is a positive/encouraging nudge
	IconType   string  // "sparkles", "trending-up", "heart", "alert"
}

// GenerateInsight analyzes transactions and returns a calm, curiosity-based insight
func GenerateInsight(transactions []database.Transaction) Insight {
	if len(transactions) == 0 {
		return Insight{
			Message:    "Welcome to BudgetMate! Start by adding some transactions to see personalized insights.",
			IsPositive: true,
			IconType:   "sparkles",
		}
	}

	// Calculate spending by category
	categorySpend := make(map[string]float64)
	var totalExpense float64

	for _, t := range transactions {
		if t.Type == "expense" {
			categorySpend[t.Category] += t.Amount
			totalExpense += t.Amount
		}
	}

	// No expenses? Celebrate!
	if totalExpense == 0 {
		return Insight{
			Message:    "No expenses recorded yet. You're off to a great start!",
			IsPositive: true,
			IconType:   "heart",
		}
	}

	// Find the top spending category
	type categoryAmount struct {
		Name   string
		Amount float64
	}
	var categories []categoryAmount
	for name, amount := range categorySpend {
		categories = append(categories, categoryAmount{name, amount})
	}
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].Amount > categories[j].Amount
	})

	topCategory := categories[0]
	percentage := (topCategory.Amount / totalExpense) * 100

	// Generate insight based on spending pattern
	if percentage > 40 {
		// High concentration - curious nudge
		return Insight{
			Message:    fmt.Sprintf("%s is %.0f%% of your spending. Was there a special occasion, or is this a pattern worth exploring?", topCategory.Name, percentage),
			Category:   topCategory.Name,
			Percentage: percentage,
			IsPositive: false,
			IconType:   "sparkles",
		}
	} else if percentage > 30 {
		// Moderate concentration - gentle nudge
		return Insight{
			Message:    fmt.Sprintf("%s leads your spending at %.0f%%. Might be worth a quick look to see if it aligns with your priorities.", topCategory.Name, percentage),
			Category:   topCategory.Name,
			Percentage: percentage,
			IsPositive: true,
			IconType:   "trending-up",
		}
	} else if len(categories) >= 3 {
		// Well balanced - positive reinforcement
		return Insight{
			Message:    "Your spending is beautifully balanced across categories. You're maintaining excellent financial discipline!",
			IsPositive: true,
			IconType:   "heart",
		}
	} else {
		// Few categories - encourage diversification awareness
		return Insight{
			Message:    "Looking good! Your expenses are focused and intentional. Keep tracking to discover more patterns.",
			IsPositive: true,
			IconType:   "sparkles",
		}
	}
}

// GenerateWeeklyInsight generates insights based on weekly spending patterns
func GenerateWeeklyInsight(transactions []database.Transaction) Insight {
	// For now, use the same logic. Can be enhanced later for time-based analysis.
	return GenerateInsight(transactions)
}

// GetSpendingTrend returns whether spending is increasing or decreasing
func GetSpendingTrend(transactions []database.Transaction) string {
	if len(transactions) < 2 {
		return "stable"
	}

	// Simple trend: compare first half to second half
	mid := len(transactions) / 2
	var firstHalf, secondHalf float64

	for i, t := range transactions {
		if t.Type == "expense" {
			if i < mid {
				firstHalf += t.Amount
			} else {
				secondHalf += t.Amount
			}
		}
	}

	if secondHalf > firstHalf*1.1 {
		return "increasing"
	} else if secondHalf < firstHalf*0.9 {
		return "decreasing"
	}
	return "stable"
}
