package reports

import (
	"fmt"
	"sort"

	"github.com/budgetmate/web/internal/database"
	"github.com/johnfercher/maroto/pkg/color"
	"github.com/johnfercher/maroto/pkg/consts"
	"github.com/johnfercher/maroto/pkg/pdf"
	"github.com/johnfercher/maroto/pkg/props"
)

// Service handles PDF report generation
type Service struct{}

// NewService creates a new report service
func NewService() *Service {
	return &Service{}
}

// GeneratePDF creates a professional PDF report from the report data
func (s *Service) GeneratePDF(data *database.ReportData) ([]byte, error) {
	m := pdf.NewMaroto(consts.Portrait, consts.A4)
	m.SetPageMargins(15, 20, 15)

	// Colors
	violetMain := color.Color{Red: 124, Green: 58, Blue: 237} // Violet-600
	slateDark := color.Color{Red: 30, Green: 41, Blue: 59}    // Slate-800
	colorGreen := color.Color{Red: 16, Green: 185, Blue: 129} // Emerald-500
	colorRed := color.Color{Red: 245, Green: 158, Blue: 11}   // Amber/Red

	// === HEADER ===
	m.RegisterHeader(func() {
		m.Row(10, func() {
			m.Col(12, func() {
				m.Text("BudgetMate", props.Text{
					Top:   0,
					Size:  16,
					Style: consts.Bold,
					Color: violetMain,
					Align: consts.Left,
				})
				m.Text("EXECUTIVE REPORT", props.Text{
					Top:   5,
					Size:  10,
					Style: consts.Bold,
					Color: slateDark,
					Align: consts.Right,
				})
			})
		})
		m.Row(10, func() {
			m.Col(12, func() {
				m.Text(fmt.Sprintf("%s %d", data.Month.String(), data.Year), props.Text{
					Top:   0,
					Size:  12,
					Color: slateDark,
					Align: consts.Right,
				})
				m.Text(data.FamilyName, props.Text{
					Top:   0,
					Size:  10,
					Color: color.Color{Red: 100, Green: 116, Blue: 139},
					Align: consts.Left,
				})
			})
		})
		m.Line(2.0, props.Line{Color: violetMain})
	})

	// === SCORECARD SECTION ===
	m.Row(15, func() {
		m.Col(12, func() {
			m.Text("FINANCIAL HEALTH CHECK", props.Text{
				Top:   10,
				Size:  12,
				Style: consts.Bold,
				Color: slateDark,
			})
		})
	})

	m.Row(40, func() {
		// Grade (Left)
		m.Col(4, func() {
			gradeColor := getGradeColor(data.Grade)
			m.Text(data.Grade, props.Text{
				Top:   0,
				Size:  50,
				Style: consts.Bold,
				Align: consts.Center,
				Color: gradeColor,
			})
			m.Text("GRADE", props.Text{
				Top:   30,
				Size:  10,
				Align: consts.Center,
				Color: color.Color{Red: 100, Green: 116, Blue: 139},
			})
		})

		// Metrics (Right)
		m.Col(8, func() {
			m.Row(20, func() {
				m.Col(4, func() {
					m.Text("INCOME", props.Text{Size: 8, Style: consts.Bold, Color: slateDark})
					m.Text(formatINR(data.TotalIncome), props.Text{Top: 8, Size: 12, Color: colorGreen})
				})
				m.Col(4, func() {
					m.Text("EXPENSES", props.Text{Size: 8, Style: consts.Bold, Color: slateDark})
					m.Text(formatINR(data.TotalExpense), props.Text{Top: 8, Size: 12, Color: colorRed})
				})
				m.Col(4, func() {
					m.Text("SAVINGS", props.Text{Size: 8, Style: consts.Bold, Color: slateDark})
					m.Text(formatINR(data.NetSavings), props.Text{Top: 8, Size: 12, Style: consts.Bold, Color: violetMain})
				})
			})
			m.Line(1.0, props.Line{Style: consts.Dashed})
			m.Row(15, func() {
				m.Col(12, func() {
					m.Text(fmt.Sprintf("Savings Rate: %.1f%%", data.SavingsRate), props.Text{
						Top: 8, Size: 11, Style: consts.Bold, Color: slateDark,
					})
				})
			})
		})
	})

	m.Line(1.0)

	// === ADVICE SECTION ===
	m.Row(25, func() {
		m.Col(12, func() {
			advice := getAdvice(data.Grade, data.SavingsRate)
			m.Text("AI INSIGHT", props.Text{Top: 5, Size: 9, Style: consts.Bold, Color: violetMain})
			m.Text(advice, props.Text{
				Top:   12,
				Size:  10,
				Style: consts.Italic,
				Color: slateDark,
			})
		})
	})

	// === SPENDING BREAKDOWN ===
	m.Row(15, func() {
		m.Col(12, func() {
			m.Text("SPENDING BREAKDOWN", props.Text{
				Top:   10,
				Size:  12,
				Style: consts.Bold,
				Color: slateDark,
			})
		})
	})

	// Sort categories by amount
	type catAmount struct {
		name   string
		amount float64
	}
	var categories []catAmount
	for name, amount := range data.CategoryBreakdown {
		categories = append(categories, catAmount{name, amount})
	}
	sort.Slice(categories, func(i, j int) bool {
		return categories[i].amount > categories[j].amount
	})

	// Table Header
	m.Row(8, func() {
		m.Col(6, func() { m.Text("CATEGORY", props.Text{Style: consts.Bold, Size: 9}) })
		m.Col(3, func() { m.Text("AMOUNT", props.Text{Style: consts.Bold, Size: 9}) })
		m.Col(3, func() { m.Text("% of TOTAL", props.Text{Style: consts.Bold, Size: 9}) })
	})
	m.Line(0.5)

	if len(categories) == 0 {
		m.Row(10, func() {
			m.Col(12, func() { m.Text("No expenses this month.", props.Text{Size: 9, Style: consts.Italic}) })
		})
	}

	for _, cat := range categories {
		percentage := float64(0)
		if data.TotalExpense > 0 {
			percentage = (cat.amount / data.TotalExpense) * 100
		}

		m.Row(8, func() {
			m.Col(6, func() { m.Text(cat.name, props.Text{Top: 2, Size: 9}) })
			m.Col(3, func() { m.Text(formatINR(cat.amount), props.Text{Top: 2, Size: 9}) })
			m.Col(3, func() { m.Text(fmt.Sprintf("%.1f%%", percentage), props.Text{Top: 2, Size: 9}) })
		})
		m.Line(0.1, props.Line{Color: color.Color{Red: 240, Green: 240, Blue: 240}})
	}

	m.Row(10, func() {}) // Spacer

	// === TOP EXPENSES ===
	m.Row(15, func() {
		m.Col(12, func() {
			m.Text("LARGEST TRANSACTIONS", props.Text{
				Top:   10,
				Size:  12,
				Style: consts.Bold,
				Color: slateDark,
			})
		})
	})

	// Table Header
	m.Row(8, func() {
		m.Col(5, func() { m.Text("DESCRIPTION", props.Text{Style: consts.Bold, Size: 9}) })
		m.Col(3, func() { m.Text("CATEGORY", props.Text{Style: consts.Bold, Size: 9}) })
		m.Col(2, func() { m.Text("DATE", props.Text{Style: consts.Bold, Size: 9}) })
		m.Col(2, func() { m.Text("AMOUNT", props.Text{Style: consts.Bold, Size: 9}) })
	})
	m.Line(0.5)

	if len(data.TopExpenses) == 0 {
		m.Row(10, func() {
			m.Col(12, func() { m.Text("No transactions found.", props.Text{Size: 9, Style: consts.Italic}) })
		})
	}

	for i, exp := range data.TopExpenses {
		if i >= 5 {
			break
		}
		m.Row(8, func() {
			m.Col(5, func() { m.Text(exp.Description, props.Text{Top: 2, Size: 9}) })
			m.Col(3, func() { m.Text(exp.Category, props.Text{Top: 2, Size: 9}) })
			m.Col(2, func() { m.Text(exp.Date.Format("02 Jan"), props.Text{Top: 2, Size: 9}) })
			m.Col(2, func() { m.Text(formatINR(exp.Amount), props.Text{Top: 2, Size: 9}) })
		})
		m.Line(0.1, props.Line{Color: color.Color{Red: 240, Green: 240, Blue: 240}})
	}

	// === FOOTER ===
	m.RegisterFooter(func() {
		m.Row(10, func() {
			m.Col(12, func() {
				m.Line(1.0)
				m.Text("Generated by BudgetMate AI • Your Personal Finance Assistant", props.Text{
					Top:   3,
					Size:  8,
					Align: consts.Center,
					Color: color.Color{Red: 148, Green: 163, Blue: 184},
				})
			})
		})
	})

	// Generate PDF bytes
	buf, err := m.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}

	return buf.Bytes(), nil
}

func getGradeColor(grade string) color.Color {
	switch grade {
	case "A":
		return color.Color{Red: 16, Green: 185, Blue: 129} // Emerald
	case "B":
		return color.Color{Red: 59, Green: 130, Blue: 246} // Blue
	case "C":
		return color.Color{Red: 245, Green: 158, Blue: 11} // Amber
	default:
		return color.Color{Red: 239, Green: 68, Blue: 68} // Red
	}
}

func getAdvice(grade string, savingsRate float64) string {
	switch grade {
	case "A":
		return "Excellent! You're saving over 20% of your income. Keep up the great work!"
	case "B":
		return "Good job! You're on track. Try to increase savings by cutting non-essential expenses."
	case "C":
		return "You're breaking even. Review your spending and identify areas to cut back."
	default:
		return "Warning: You're spending more than you earn. Review all expenses immediately."
	}
}

func formatINR(amount float64) string {
	if amount < 0 {
		return fmt.Sprintf("-₹%.0f", -amount)
	}
	if amount >= 10000000 { // 1 crore
		return fmt.Sprintf("₹%.2f Cr", amount/10000000)
	}
	if amount >= 100000 { // 1 lakh
		return fmt.Sprintf("₹%.2f L", amount/100000)
	}
	return fmt.Sprintf("₹%.0f", amount)
}
