package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Service handles "Smart" categorization using Hybrid (Groq API + Rule-Based Fallback)
type Service struct {
	Client *http.Client
}

func NewService() *Service {
	return &Service{
		Client: &http.Client{Timeout: 10 * time.Second},
	}
}

// CategorizeTransaction attempts to use Groq API, falls back to Rules
func (s *Service) CategorizeTransaction(description string) (string, error) {
	// 1. Try Groq API if key is available
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey != "" {
		category, err := s.callGroq(apiKey, description)
		if err == nil {
			return category, nil
		}
		fmt.Printf("Groq API failed: %v. Falling back to rules.\n", err)
	}

	// 2. Rule-Based Fallback (Offline / No Key / Error)
	return s.categorizeByRules(description), nil
}

// Groq API Logic
type GroqRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GroqResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// callGroqGeneric makes a generic call to Groq API with custom messages
func (s *Service) callGroqGeneric(apiKey string, messages []Message) (string, error) {
	reqBody := GroqRequest{
		Model:    "llama-3.1-8b-instant", // Updated to current free-tier model
		Messages: messages,
	}

	jsonBody, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("groq status %d: %s", resp.StatusCode, string(body))
	}

	var groqResp GroqResponse
	if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil {
		return "", err
	}

	if len(groqResp.Choices) > 0 {
		return strings.TrimSpace(groqResp.Choices[0].Message.Content), nil
	}
	return "", fmt.Errorf("empty response")
}

func (s *Service) callGroq(apiKey, description string) (string, error) {
	prompt := fmt.Sprintf("Categorize this transaction '%s' into exactly one of: [Food & Dining, Groceries, Transportation, Utilities, Entertainment, Healthcare, Shopping, Salary, Investment]. Return ONLY the category name.", description)

	messages := []Message{
		{Role: "user", Content: prompt},
	}

	return s.callGroqGeneric(apiKey, messages)
}

// ChatMessage represents a message in the conversation
type ChatMessage struct {
	Role    string // "user" or "assistant"
	Content string
}

// GenerateFinancialAdvice generates AI-powered financial advice based on user's transaction history
func (s *Service) GenerateFinancialAdvice(history string, question string, conversationHistory []ChatMessage) (string, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "I'm sorry, but AI features are currently unavailable. Please check your GROQ_API_KEY configuration.", nil
	}

	systemPrompt := fmt.Sprintf(`You are BudgetMate, a friendly and REALISTIC financial advisor. You remember our entire conversation.

=== USER'S FINANCIAL DATA (Last 30 Days) ===
%s
=== END DATA ===

CRITICAL RULES:
1. REMEMBER the conversation history - if user mentioned changing income, use that new number
2. Calculate: Savings Potential = Income - Expenses (MAX they can save)
3. NEVER suggest saving more than Savings Potential
4. If user gives hypothetical scenarios (like "what if my income is X"), use those numbers
5. Be consistent with previous answers in this conversation

MATH:
- Timeline = Goal Amount ÷ Monthly Savings
- Be realistic about expensive goals (cars, houses)

RESPONSE STYLE:
- Natural, friendly conversation
- Remember what we discussed earlier
- Keep it short (80-120 words)
- Each point on NEW LINE
- Format: ₹XX,XXX or ₹XX lakhs`, history)

	// Build messages with conversation history
	messages := []Message{
		{Role: "system", Content: systemPrompt},
	}

	// Add conversation history
	for _, msg := range conversationHistory {
		messages = append(messages, Message{Role: msg.Role, Content: msg.Content})
	}

	// Add current question
	messages = append(messages, Message{Role: "user", Content: question})

	response, err := s.callGroqGeneric(apiKey, messages)
	if err != nil {
		fmt.Printf("🔴 AI Service Error: %v\n", err)
		return "", fmt.Errorf("AI service error: %w", err)
	}

	return response, nil
}

// Rule-Based Logic (0 RAM usage)
func (s *Service) categorizeByRules(description string) string {
	desc := strings.ToLower(description)

	if containsAny(desc, "swiggy", "zomato", "eats", "food", "burger", "pizza", "coffee", "cafe", "starbucks", "mcd", "kfc", "restaurant", "dining", "lunch", "dinner") {
		return "Food & Dining"
	}
	if containsAny(desc, "grocery", "mart", "vegetable", "fruit", "milk", "bigbasket", "blinkit", "zepto", "instamart", "dmart") {
		return "Groceries"
	}
	if containsAny(desc, "uber", "ola", "rapido", "cab", "taxi", "bus", "metro", "train", "flight", "air", "fuel", "petrol", "shell", "parking", "toll") {
		return "Transportation"
	}
	if containsAny(desc, "electricity", "bescom", "power", "water", "gas", "internet", "wifi", "jio", "airtel", "recharge", "bill") {
		return "Utilities"
	}
	if containsAny(desc, "netflix", "prime", "hotstar", "spotify", "movie", "cinema", "game", "steam") {
		return "Entertainment"
	}
	if containsAny(desc, "amazon", "flipkart", "myntra", "zara", "h&m", "shopping", "store", "mall") {
		return "Shopping"
	}
	if containsAny(desc, "pharmacy", "doctor", "hospital", "apollo", "medplus", "medicine") {
		return "Healthcare"
	}
	if containsAny(desc, "zerodha", "groww", "sip", "invest", "stock") {
		return "Investment"
	} // missing Salary but covered by Other or added if needed

	return "Other"
}

func containsAny(s string, keywords ...string) bool {
	for _, k := range keywords {
		if strings.Contains(s, k) {
			return true
		}
	}
	return false
}
