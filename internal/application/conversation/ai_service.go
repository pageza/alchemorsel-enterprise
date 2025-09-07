package conversation

import (
	"context"
	"fmt"
	"log"
	"strings"
)

// AIService handles AI interactions for conversations
type AIService struct {
	ollamaClient OllamaClient
	openAIClient OpenAIClient
}

// OllamaClient interface for Ollama integration
type OllamaClient interface {
	GenerateChatCompletion(ctx context.Context, messages []ChatMessage) (string, error)
	HealthCheck(ctx context.Context) error
}

// OpenAIClient interface for OpenAI integration (fallback)
type OpenAIClient interface {
	GenerateChatCompletion(ctx context.Context, messages []ChatMessage) (string, error)
}

// ChatMessage represents a chat message for AI services
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ConversationalResponse represents the AI's response structure
type ConversationalResponse struct {
	Content     string                 `json:"content"`
	Intent      ConversationIntent     `json:"intent"`
	Confidence  float64                `json:"confidence"`
	Metadata    map[string]interface{} `json:"metadata"`
	NextActions []string               `json:"next_actions,omitempty"`
}

// RecipeCreationContext holds context for recipe creation workflows
type RecipeCreationContext struct {
	CurrentStep  string   `json:"current_step"`
	RecipeTitle  string   `json:"recipe_title,omitempty"`
	Ingredients  []string `json:"ingredients,omitempty"`
	Instructions []string `json:"instructions,omitempty"`
	Cuisine      string   `json:"cuisine,omitempty"`
	Difficulty   string   `json:"difficulty,omitempty"`
	ServingSize  int      `json:"serving_size,omitempty"`
	DietaryReqs  []string `json:"dietary_requirements,omitempty"`
	CookingTime  int      `json:"cooking_time,omitempty"`
	MissingInfo  []string `json:"missing_info,omitempty"`
}

// NewAIService creates a new AI service
func NewAIService(ollamaClient OllamaClient, openAIClient OpenAIClient) *AIService {
	return &AIService{
		ollamaClient: ollamaClient,
		openAIClient: openAIClient,
	}
}

// GenerateConversationalResponse generates an AI response for a conversation
func (s *AIService) GenerateConversationalResponse(ctx context.Context, conversation *Conversation, messages []*Message, userMessage string) (*ConversationalResponse, error) {
	// Build conversation history
	chatMessages := s.buildChatHistory(conversation, messages, userMessage)

	// Generate response using primary AI service (Ollama)
	response, err := s.generateWithOllama(ctx, chatMessages, conversation.Intent)
	if err != nil {
		log.Printf("Ollama generation failed, trying OpenAI fallback: %v", err)
		response, err = s.generateWithOpenAI(ctx, chatMessages, conversation.Intent)
		if err != nil {
			return s.generateFallbackResponse(conversation.Intent, userMessage), nil
		}
	}

	return response, nil
}

// buildChatHistory constructs the chat history for the AI
func (s *AIService) buildChatHistory(conversation *Conversation, messages []*Message, userMessage string) []ChatMessage {
	var chatMessages []ChatMessage

	// Add system prompt based on conversation intent
	systemPrompt := s.buildSystemPrompt(conversation.Intent)
	chatMessages = append(chatMessages, ChatMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	// Add conversation history (limit to last 10 messages for context)
	startIdx := 0
	if len(messages) > 10 {
		startIdx = len(messages) - 10
	}

	for i := startIdx; i < len(messages); i++ {
		msg := messages[i]
		role := "user"
		if msg.Role == RoleAssistant {
			role = "assistant"
		}
		chatMessages = append(chatMessages, ChatMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	// Add current user message
	chatMessages = append(chatMessages, ChatMessage{
		Role:    "user",
		Content: userMessage,
	})

	return chatMessages
}

// buildSystemPrompt creates a system prompt based on conversation intent
func (s *AIService) buildSystemPrompt(intent ConversationIntent) string {
	basePrompt := `You are an expert AI chef assistant called "AI Chef" helping users with cooking and recipes. You are knowledgeable, friendly, and practical. Always provide helpful, accurate, and safe cooking advice.`

	switch intent {
	case IntentRecipeCreation:
		return basePrompt + `

RECIPE CREATION MODE: You are helping create a complete recipe. Follow this workflow:

1. GATHER INFORMATION: Ask for missing details if needed:
   - What dish they want to make
   - Dietary restrictions or preferences
   - Number of servings
   - Available ingredients or preferred ingredients
   - Cooking time constraints
   - Skill level

2. CREATE RECIPE: Once you have enough information, provide a complete recipe with:
   - Clear title
   - Brief description
   - Ingredient list with measurements
   - Step-by-step instructions
   - Cooking times and temperatures
   - Serving information
   - Tips or variations

3. REFINE: Be ready to modify based on feedback.

Keep responses conversational and engaging. Ask follow-up questions to create the perfect recipe.`

	case IntentCookingHelp:
		return basePrompt + `

COOKING HELP MODE: You are answering cooking questions and providing techniques. Focus on:
- Clear, practical explanations
- Safety tips when relevant
- Alternative methods when applicable
- Troubleshooting common problems
- Encouraging the user to try new techniques

Be specific and actionable in your advice.`

	case IntentIngredientSubst:
		return basePrompt + `

INGREDIENT SUBSTITUTION MODE: Help users find alternatives for ingredients. Consider:
- Similar flavor profiles
- Texture compatibility
- Cooking behavior
- Availability and cost
- Dietary restrictions
- Measurement conversions

Always explain WHY the substitution works and any adjustments needed.`

	case IntentMealPlanning:
		return basePrompt + `

MEAL PLANNING MODE: Help users plan meals efficiently. Consider:
- Nutritional balance
- Time constraints
- Budget considerations
- Dietary preferences
- Ingredient reuse across meals
- Prep-ahead options
- Seasonal ingredients

Provide practical, organized meal plans with shopping lists when helpful.`

	default:
		return basePrompt + `

GENERAL COOKING ASSISTANT MODE: You can help with any cooking-related questions, recipe requests, meal planning, ingredient substitutions, and cooking techniques. Ask clarifying questions to provide the most helpful response.`
	}
}

// generateWithOllama generates response using Ollama
func (s *AIService) generateWithOllama(ctx context.Context, messages []ChatMessage, intent ConversationIntent) (*ConversationalResponse, error) {
	// Check if Ollama client is nil
	if s.ollamaClient == nil {
		return nil, fmt.Errorf("ollama client is nil")
	}

	// Check if Ollama is available
	if err := s.ollamaClient.HealthCheck(ctx); err != nil {
		return nil, fmt.Errorf("ollama not available: %w", err)
	}

	response, err := s.ollamaClient.GenerateChatCompletion(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("ollama generation failed: %w", err)
	}

	return &ConversationalResponse{
		Content:    response,
		Intent:     intent,
		Confidence: 0.85, // Higher confidence for local AI
		Metadata: map[string]interface{}{
			"provider": "ollama",
			"model":    "local",
		},
	}, nil
}

// generateWithOpenAI generates response using OpenAI (fallback)
func (s *AIService) generateWithOpenAI(ctx context.Context, messages []ChatMessage, intent ConversationIntent) (*ConversationalResponse, error) {
	if s.openAIClient == nil {
		return nil, fmt.Errorf("openai client not available")
	}

	response, err := s.openAIClient.GenerateChatCompletion(ctx, messages)
	if err != nil {
		return nil, fmt.Errorf("openai generation failed: %w", err)
	}

	return &ConversationalResponse{
		Content:    response,
		Intent:     intent,
		Confidence: 0.75,
		Metadata: map[string]interface{}{
			"provider": "openai",
			"model":    "gpt-3.5-turbo",
		},
	}, nil
}

// generateFallbackResponse creates a fallback response when AI services are unavailable
func (s *AIService) generateFallbackResponse(intent ConversationIntent, userMessage string) *ConversationalResponse {
	var content string

	switch intent {
	case IntentRecipeCreation:
		content = `I'd love to help you create a recipe! However, I'm having trouble accessing my AI capabilities right now. 

Here's what I can suggest:
- Try describing the dish you want to make in more detail
- Let me know what ingredients you have available
- Specify any dietary restrictions or preferences
- Tell me how much time you have for cooking

Once my AI is back online, I'll be able to create a detailed recipe for you!`

	case IntentCookingHelp:
		content = `I'm here to help with your cooking question! While my AI is temporarily unavailable, I can suggest:
- Check reputable cooking websites like Serious Eats or America's Test Kitchen
- Look up the technique on cooking YouTube channels
- Try cooking forums like Reddit's r/Cooking for community advice
- Consult classic cookbooks for fundamental techniques

I'll be back online soon to give you personalized cooking advice!`

	case IntentIngredientSubst:
		content = `I'd be happy to help with ingredient substitutions! While my AI is temporarily down, here are some common substitutions:
- Butter → Oil (3/4 the amount)
- Sugar → Honey (3/4 the amount, reduce liquid)
- Eggs → Flax eggs (1 tbsp ground flax + 3 tbsp water per egg)
- Milk → Plant milk (1:1 ratio)

For specific substitutions, try online substitution calculators or cooking websites until I'm back online!`

	case IntentMealPlanning:
		content = `I'd love to help you plan meals! While my AI is temporarily unavailable, consider:
- Plan around what's in season
- Choose one day for meal prep
- Plan meals that share ingredients
- Balance proteins, vegetables, and grains
- Consider your weekly schedule for cooking time

I'll be back soon to create personalized meal plans for you!`

	default:
		content = `I'm your AI chef assistant, and I'd love to help you with cooking! I'm experiencing some technical difficulties right now, but I'll be back online soon.

In the meantime, feel free to browse our recipe collection or try asking your question again in a moment. I'm here to help with recipes, cooking techniques, meal planning, and any culinary questions you have!`
	}

	return &ConversationalResponse{
		Content:    content,
		Intent:     intent,
		Confidence: 0.5, // Lower confidence for fallback
		Metadata: map[string]interface{}{
			"provider": "fallback",
			"reason":   "ai_services_unavailable",
		},
	}
}

// ExtractRecipeFromConversation analyzes a conversation to extract recipe information
func (s *AIService) ExtractRecipeFromConversation(ctx context.Context, messages []*Message) (*RecipeCreationContext, error) {
	// Analyze messages to extract recipe components
	context := &RecipeCreationContext{
		CurrentStep:  "gathering_info",
		Ingredients:  make([]string, 0),
		Instructions: make([]string, 0),
		DietaryReqs:  make([]string, 0),
		MissingInfo:  make([]string, 0),
	}

	// Simple extraction logic - in a real implementation, this would use NLP
	for _, msg := range messages {
		if msg.Role == RoleUser {
			content := strings.ToLower(msg.Content)

			// Extract dish name
			if context.RecipeTitle == "" {
				context.RecipeTitle = s.extractDishName(content)
			}

			// Extract ingredients
			ingredients := s.extractIngredients(content)
			for _, ing := range ingredients {
				if !contains(context.Ingredients, ing) {
					context.Ingredients = append(context.Ingredients, ing)
				}
			}

			// Extract dietary requirements
			dietary := s.extractDietaryRequirements(content)
			for _, req := range dietary {
				if !contains(context.DietaryReqs, req) {
					context.DietaryReqs = append(context.DietaryReqs, req)
				}
			}

			// Extract serving size
			if servings := s.extractServingSize(content); servings > 0 {
				context.ServingSize = servings
			}

			// Extract cooking time
			if cookTime := s.extractCookingTime(content); cookTime > 0 {
				context.CookingTime = cookTime
			}
		}
	}

	// Determine what information is still needed
	context.MissingInfo = s.identifyMissingInfo(context)

	// Determine current step
	if len(context.MissingInfo) == 0 && context.RecipeTitle != "" {
		context.CurrentStep = "ready_to_create"
	} else if context.RecipeTitle != "" {
		context.CurrentStep = "gathering_details"
	} else {
		context.CurrentStep = "gathering_info"
	}

	return context, nil
}

// Helper functions for extraction
func (s *AIService) extractDishName(content string) string {
	patterns := []string{
		"recipe for ", "make ", "cook ", "prepare ", "create ",
		"want to make ", "how to make ", "recipe ",
	}

	for _, pattern := range patterns {
		if idx := strings.Index(content, pattern); idx != -1 {
			start := idx + len(pattern)
			end := start + 30 // Limit length
			if end > len(content) {
				end = len(content)
			}

			dishName := strings.TrimSpace(content[start:end])
			// Clean up the dish name
			if stopIdx := strings.IndexAny(dishName, ".,!?"); stopIdx != -1 {
				dishName = dishName[:stopIdx]
			}

			if len(dishName) > 3 && len(dishName) < 50 {
				return strings.Title(dishName)
			}
		}
	}

	return ""
}

func (s *AIService) extractIngredients(content string) []string {
	// Simple pattern matching - in a real implementation, use NLP
	ingredients := []string{}
	commonIngredients := []string{
		"chicken", "beef", "pork", "fish", "salmon", "tuna",
		"pasta", "rice", "noodles", "bread",
		"tomatoes", "onions", "garlic", "mushrooms", "peppers",
		"cheese", "milk", "eggs", "butter", "oil",
		"basil", "oregano", "thyme", "parsley", "cilantro",
	}

	for _, ingredient := range commonIngredients {
		if strings.Contains(content, ingredient) {
			ingredients = append(ingredients, ingredient)
		}
	}

	return ingredients
}

func (s *AIService) extractDietaryRequirements(content string) []string {
	dietary := []string{}
	requirements := map[string]string{
		"vegetarian": "vegetarian",
		"vegan":      "vegan",
		"gluten":     "gluten-free",
		"dairy":      "dairy-free",
		"keto":       "keto",
		"low carb":   "low-carb",
		"paleo":      "paleo",
		"healthy":    "healthy",
	}

	for keyword, req := range requirements {
		if strings.Contains(content, keyword) {
			dietary = append(dietary, req)
		}
	}

	return dietary
}

func (s *AIService) extractServingSize(content string) int {
	// Simple pattern matching for numbers followed by "serving" or "people"
	patterns := []string{"serving", "people", "person"}

	for _, pattern := range patterns {
		if idx := strings.Index(content, pattern); idx != -1 {
			// Look backwards for a number
			for i := idx - 1; i >= 0; i-- {
				if content[i] >= '0' && content[i] <= '9' {
					return int(content[i] - '0')
				}
				if content[i] != ' ' {
					break
				}
			}
		}
	}

	return 0
}

func (s *AIService) extractCookingTime(content string) int {
	// Look for patterns like "30 minutes", "1 hour"
	if strings.Contains(content, "minute") && strings.Contains(content, "30") {
		return 30
	}
	if strings.Contains(content, "hour") {
		return 60
	}
	if strings.Contains(content, "quick") {
		return 15
	}

	return 0
}

func (s *AIService) identifyMissingInfo(context *RecipeCreationContext) []string {
	missing := []string{}

	if context.RecipeTitle == "" {
		missing = append(missing, "dish_name")
	}
	if len(context.Ingredients) == 0 {
		missing = append(missing, "ingredients")
	}
	if context.ServingSize == 0 {
		missing = append(missing, "serving_size")
	}

	return missing
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
