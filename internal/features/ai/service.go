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

func (s *Service) callGroq(apiKey, description string) (string, error) {
	prompt := fmt.Sprintf("Categorize this transaction '%s' into exactly one of: [Food & Dining, Groceries, Transportation, Utilities, Entertainment, Healthcare, Shopping, Salary, Investment]. Return ONLY the category name.", description)

	reqBody := GroqRequest{
		Model: "llama3-8b-8192", // Fast, free-tier friendly model on Groq
		Messages: []Message{
			{Role: "user", Content: prompt},
		},
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
