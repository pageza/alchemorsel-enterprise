package testutils

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/application/conversation"
	"github.com/google/uuid"
)

// ConversationFixture represents a test conversation fixture
type ConversationFixture struct {
	Conversation *conversation.Conversation
	Messages     []*conversation.Message
	Context      []*conversation.ConversationContext
}

// ConversationScenario represents a predefined conversation scenario
type ConversationScenario struct {
	Name         string
	UserID       string
	Intent       conversation.ConversationIntent
	MessageFlow  []ScenarioMessageFlow
	ExpectedFlow []string
	Metadata     map[string]interface{}
}

// ScenarioMessageFlow represents a message exchange in a scenario
type ScenarioMessageFlow struct {
	UserMessage      string
	ExpectedResponse string
	ContextUpdates   map[string]interface{}
	Delay           time.Duration
}

// CreateRecipeCreationFixture creates a complete recipe creation conversation fixture
func CreateRecipeCreationFixture() *ConversationFixture {
	userID := "fixture-user-" + uuid.New().String()
	conversationID := uuid.New().String()
	
	conv := &conversation.Conversation{
		ID:     conversationID,
		UserID: userID,
		Title:  "Recipe: Pasta Carbonara",
		Intent: conversation.IntentRecipeCreation,
		Status: conversation.StatusActive,
		Metadata: map[string]interface{}{
			"source":     "test_fixture",
			"complexity": "intermediate",
			"cuisine":    "italian",
		},
		CreatedAt: time.Now().Add(-30 * time.Minute),
		UpdatedAt: time.Now().Add(-5 * time.Minute),
	}
	
	messages := []*conversation.Message{
		{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           conversation.RoleUser,
			Content:        "I want to create a recipe for pasta carbonara",
			Metadata:       make(map[string]interface{}),
			TokensUsed:     12,
			CreatedAt:      time.Now().Add(-30 * time.Minute),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           conversation.RoleAssistant,
			Content:        "Great choice! Carbonara is a classic Roman dish. How many servings would you like this recipe to make?",
			Metadata: map[string]interface{}{
				"ai_provider": "test",
				"confidence":  0.9,
			},
			TokensUsed:       28,
			ProcessingTimeMs: 150,
			ModelUsed:        "test-model",
			CreatedAt:        time.Now().Add(-29 * time.Minute),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           conversation.RoleUser,
			Content:        "4 servings please",
			Metadata:       make(map[string]interface{}),
			TokensUsed:     3,
			CreatedAt:      time.Now().Add(-28 * time.Minute),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           conversation.RoleAssistant,
			Content:        "Perfect! Do you have any dietary restrictions I should know about?",
			Metadata: map[string]interface{}{
				"ai_provider": "test",
				"confidence":  0.85,
			},
			TokensUsed:       15,
			ProcessingTimeMs: 120,
			ModelUsed:        "test-model",
			CreatedAt:        time.Now().Add(-27 * time.Minute),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           conversation.RoleUser,
			Content:        "No dietary restrictions",
			Metadata:       make(map[string]interface{}),
			TokensUsed:     4,
			CreatedAt:      time.Now().Add(-26 * time.Minute),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           conversation.RoleAssistant,
			Content:        "Excellent! Here's a traditional carbonara recipe for 4 servings: [Recipe details would follow...]",
			Metadata: map[string]interface{}{
				"ai_provider": "test",
				"confidence":  0.95,
				"recipe_generated": true,
			},
			TokensUsed:       45,
			ProcessingTimeMs: 200,
			ModelUsed:        "test-model",
			CreatedAt:        time.Now().Add(-25 * time.Minute),
		},
	}
	
	contexts := []*conversation.ConversationContext{
		{
			ConversationID: conversationID,
			ContextType:    "recipe_creation",
			ContextData: map[string]interface{}{
				"dish_name":    "pasta carbonara",
				"servings":     4,
				"progress":     "completed",
				"ingredients":  []string{"pasta", "eggs", "pecorino", "guanciale", "black pepper"},
				"cuisine_type": "italian",
			},
			CreatedAt: time.Now().Add(-30 * time.Minute),
			UpdatedAt: time.Now().Add(-25 * time.Minute),
		},
		{
			ConversationID: conversationID,
			ContextType:    "ai_metadata",
			ContextData: map[string]interface{}{
				"total_tokens":     107,
				"avg_response_time": 156,
				"model_used":       "test-model",
				"provider":         "test",
			},
			CreatedAt: time.Now().Add(-25 * time.Minute),
			UpdatedAt: time.Now().Add(-25 * time.Minute),
		},
	}
	
	return &ConversationFixture{
		Conversation: conv,
		Messages:     messages,
		Context:      contexts,
	}
}

// CreateCookingHelpFixture creates a cooking help conversation fixture
func CreateCookingHelpFixture() *ConversationFixture {
	userID := "fixture-user-" + uuid.New().String()
	conversationID := uuid.New().String()
	
	conv := &conversation.Conversation{
		ID:     conversationID,
		UserID: userID,
		Title:  "Cooking Help",
		Intent: conversation.IntentCookingHelp,
		Status: conversation.StatusActive,
		Metadata: map[string]interface{}{
			"source": "test_fixture",
			"topic":  "steak_cooking",
		},
		CreatedAt: time.Now().Add(-15 * time.Minute),
		UpdatedAt: time.Now().Add(-2 * time.Minute),
	}
	
	messages := []*conversation.Message{
		{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           conversation.RoleUser,
			Content:        "How do I properly sear a steak?",
			Metadata:       make(map[string]interface{}),
			TokensUsed:     8,
			CreatedAt:      time.Now().Add(-15 * time.Minute),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           conversation.RoleAssistant,
			Content:        "To properly sear a steak, you need high heat and a hot pan. Pat the steak dry first, season it, then sear for 2-3 minutes per side without moving it.",
			Metadata: map[string]interface{}{
				"ai_provider": "test",
				"confidence":  0.92,
			},
			TokensUsed:       42,
			ProcessingTimeMs: 180,
			ModelUsed:        "test-model",
			CreatedAt:        time.Now().Add(-14 * time.Minute),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           conversation.RoleUser,
			Content:        "What temperature should the pan be?",
			Metadata:       make(map[string]interface{}),
			TokensUsed:     7,
			CreatedAt:      time.Now().Add(-13 * time.Minute),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           conversation.RoleAssistant,
			Content:        "The pan should be very hot - around 400-450°F (200-230°C). You can test it by dropping a bit of water; it should sizzle and evaporate immediately.",
			Metadata: map[string]interface{}{
				"ai_provider": "test",
				"confidence":  0.88,
			},
			TokensUsed:       35,
			ProcessingTimeMs: 140,
			ModelUsed:        "test-model",
			CreatedAt:        time.Now().Add(-12 * time.Minute),
		},
	}
	
	contexts := []*conversation.ConversationContext{
		{
			ConversationID: conversationID,
			ContextType:    "cooking_help",
			ContextData: map[string]interface{}{
				"topic":        "steak_searing",
				"skill_level":  "beginner",
				"help_type":    "technique",
				"equipment":    []string{"pan", "stove"},
			},
			CreatedAt: time.Now().Add(-15 * time.Minute),
			UpdatedAt: time.Now().Add(-12 * time.Minute),
		},
	}
	
	return &ConversationFixture{
		Conversation: conv,
		Messages:     messages,
		Context:      contexts,
	}
}

// CreateIngredientSubstitutionFixture creates an ingredient substitution fixture
func CreateIngredientSubstitutionFixture() *ConversationFixture {
	userID := "fixture-user-" + uuid.New().String()
	conversationID := uuid.New().String()
	
	conv := &conversation.Conversation{
		ID:     conversationID,
		UserID: userID,
		Title:  "Ingredient Substitution",
		Intent: conversation.IntentIngredientSubst,
		Status: conversation.StatusActive,
		Metadata: map[string]interface{}{
			"source":            "test_fixture",
			"substitution_type": "dairy",
		},
		CreatedAt: time.Now().Add(-10 * time.Minute),
		UpdatedAt: time.Now().Add(-1 * time.Minute),
	}
	
	messages := []*conversation.Message{
		{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           conversation.RoleUser,
			Content:        "I don't have heavy cream for my recipe. What can I substitute?",
			Metadata:       make(map[string]interface{}),
			TokensUsed:     14,
			CreatedAt:      time.Now().Add(-10 * time.Minute),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           conversation.RoleAssistant,
			Content:        "You can substitute heavy cream with several options: milk + butter, half-and-half, or coconut cream. What are you making?",
			Metadata: map[string]interface{}{
				"ai_provider": "test",
				"confidence":  0.91,
			},
			TokensUsed:       32,
			ProcessingTimeMs: 165,
			ModelUsed:        "test-model",
			CreatedAt:        time.Now().Add(-9 * time.Minute),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           conversation.RoleUser,
			Content:        "I'm making alfredo sauce",
			Metadata:       make(map[string]interface{}),
			TokensUsed:     5,
			CreatedAt:      time.Now().Add(-8 * time.Minute),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           conversation.RoleAssistant,
			Content:        "For alfredo sauce, mix 3/4 cup milk with 1/4 cup melted butter. This will give you 1 cup of heavy cream substitute that works perfectly for alfredo!",
			Metadata: map[string]interface{}{
				"ai_provider": "test",
				"confidence":  0.94,
			},
			TokensUsed:       38,
			ProcessingTimeMs: 155,
			ModelUsed:        "test-model",
			CreatedAt:        time.Now().Add(-7 * time.Minute),
		},
	}
	
	contexts := []*conversation.ConversationContext{
		{
			ConversationID: conversationID,
			ContextType:    "substitution",
			ContextData: map[string]interface{}{
				"original_ingredient": "heavy cream",
				"recipe_type":        "alfredo sauce",
				"substitution":       "milk + butter",
				"ratio":              "3:1",
				"success":            true,
			},
			CreatedAt: time.Now().Add(-10 * time.Minute),
			UpdatedAt: time.Now().Add(-7 * time.Minute),
		},
	}
	
	return &ConversationFixture{
		Conversation: conv,
		Messages:     messages,
		Context:      contexts,
	}
}

// CreateMealPlanningFixture creates a meal planning conversation fixture
func CreateMealPlanningFixture() *ConversationFixture {
	userID := "fixture-user-" + uuid.New().String()
	conversationID := uuid.New().String()
	
	conv := &conversation.Conversation{
		ID:     conversationID,
		UserID: userID,
		Title:  "Meal Planning",
		Intent: conversation.IntentMealPlanning,
		Status: conversation.StatusActive,
		Metadata: map[string]interface{}{
			"source":    "test_fixture",
			"timeframe": "weekly",
			"people":    2,
		},
		CreatedAt: time.Now().Add(-45 * time.Minute),
		UpdatedAt: time.Now().Add(-10 * time.Minute),
	}
	
	messages := []*conversation.Message{
		{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           conversation.RoleUser,
			Content:        "Help me plan meals for next week for 2 people",
			Metadata:       make(map[string]interface{}),
			TokensUsed:     10,
			CreatedAt:      time.Now().Add(-45 * time.Minute),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           conversation.RoleAssistant,
			Content:        "I'd be happy to help plan your meals! What are your dietary preferences and do you have any time constraints for cooking?",
			Metadata: map[string]interface{}{
				"ai_provider": "test",
				"confidence":  0.87,
			},
			TokensUsed:       30,
			ProcessingTimeMs: 170,
			ModelUsed:        "test-model",
			CreatedAt:        time.Now().Add(-44 * time.Minute),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           conversation.RoleUser,
			Content:        "We eat everything but prefer healthy options. Quick weeknight meals would be great",
			Metadata:       make(map[string]interface{}),
			TokensUsed:     15,
			CreatedAt:      time.Now().Add(-43 * time.Minute),
		},
		{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           conversation.RoleAssistant,
			Content:        "Perfect! Here's a healthy weekly meal plan with quick options: [Meal plan details would follow...]",
			Metadata: map[string]interface{}{
				"ai_provider": "test",
				"confidence":  0.93,
				"plan_generated": true,
			},
			TokensUsed:       25,
			ProcessingTimeMs: 185,
			ModelUsed:        "test-model",
			CreatedAt:        time.Now().Add(-42 * time.Minute),
		},
	}
	
	contexts := []*conversation.ConversationContext{
		{
			ConversationID: conversationID,
			ContextType:    "meal_planning",
			ContextData: map[string]interface{}{
				"timeframe":    "weekly",
				"people_count": 2,
				"preferences":  []string{"healthy", "quick"},
				"constraints":  []string{"weeknight cooking"},
				"plan_type":    "balanced",
			},
			CreatedAt: time.Now().Add(-45 * time.Minute),
			UpdatedAt: time.Now().Add(-42 * time.Minute),
		},
	}
	
	return &ConversationFixture{
		Conversation: conv,
		Messages:     messages,
		Context:      contexts,
	}
}

// CreateComplexConversationFixture creates a complex multi-turn conversation
func CreateComplexConversationFixture() *ConversationFixture {
	userID := "fixture-user-" + uuid.New().String()
	conversationID := uuid.New().String()
	
	conv := &conversation.Conversation{
		ID:     conversationID,
		UserID: userID,
		Title:  "Recipe: Beef Wellington",
		Intent: conversation.IntentRecipeCreation,
		Status: conversation.StatusActive,
		Metadata: map[string]interface{}{
			"source":     "test_fixture",
			"complexity": "advanced",
			"cuisine":    "french",
			"special":    "complex_recipe",
		},
		CreatedAt: time.Now().Add(-2 * time.Hour),
		UpdatedAt: time.Now().Add(-30 * time.Minute),
	}
	
	// Create a longer conversation with 20 messages
	messages := make([]*conversation.Message, 20)
	baseTime := time.Now().Add(-2 * time.Hour)
	
	messageContents := []struct {
		role    conversation.MessageRole
		content string
	}{
		{conversation.RoleUser, "I want to create a beef wellington recipe for a special dinner"},
		{conversation.RoleAssistant, "Beef Wellington is an impressive dish! How many people will you be serving?"},
		{conversation.RoleUser, "8 people"},
		{conversation.RoleAssistant, "Perfect! Do you have experience with pastry and pâté?"},
		{conversation.RoleUser, "I'm comfortable with pastry but never made pâté"},
		{conversation.RoleAssistant, "No problem! I can guide you through making a mushroom duxelles instead. What grade of beef can you source?"},
		{conversation.RoleUser, "I can get prime beef tenderloin"},
		{conversation.RoleAssistant, "Excellent! Here's what we'll need: prime beef tenderloin, puff pastry, mushrooms, shallots, herbs..."},
		{conversation.RoleUser, "Should I make the puff pastry from scratch?"},
		{conversation.RoleAssistant, "For this recipe, high-quality store-bought puff pastry works perfectly and saves time."},
		{conversation.RoleUser, "What about the mushroom mixture?"},
		{conversation.RoleAssistant, "The duxelles is key! Finely chop 2 lbs mixed mushrooms and cook until all moisture evaporates..."},
		{conversation.RoleUser, "How do I prevent soggy bottom?"},
		{conversation.RoleAssistant, "Great question! Pre-bake the pastry base and ensure the duxelles is completely dry."},
		{conversation.RoleUser, "What internal temperature for the beef?"},
		{conversation.RoleAssistant, "For medium-rare, aim for 125°F internal temperature after resting."},
		{conversation.RoleUser, "How long should it rest?"},
		{conversation.RoleAssistant, "Rest for 10-15 minutes before slicing to allow juices to redistribute."},
		{conversation.RoleUser, "Can I prepare anything ahead?"},
		{conversation.RoleAssistant, "Yes! Make the duxelles a day ahead, and assemble the wellington morning of serving."},
		{conversation.RoleUser, "Perfect! Can you give me the complete recipe with timeline?"},
		{conversation.RoleAssistant, "Absolutely! Here's your complete Beef Wellington recipe with preparation timeline..."},
	}
	
	for i, msgContent := range messageContents {
		if i >= len(messages) {
			break
		}
		
		messages[i] = &conversation.Message{
			ID:             uuid.New().String(),
			ConversationID: conversationID,
			Role:           msgContent.role,
			Content:        msgContent.content,
			Metadata:       make(map[string]interface{}),
			TokensUsed:     len(strings.Fields(msgContent.content)),
			ProcessingTimeMs: 150 + (i * 10), // Increasing processing time
			ModelUsed:        "test-model",
			CreatedAt:        baseTime.Add(time.Duration(i*3) * time.Minute),
		}
		
		if msgContent.role == conversation.RoleAssistant {
			messages[i].Metadata = map[string]interface{}{
				"ai_provider": "test",
				"confidence":  0.85 + float64(i)*0.005, // Increasing confidence
			}
		}
	}
	
	contexts := []*conversation.ConversationContext{
		{
			ConversationID: conversationID,
			ContextType:    "recipe_creation",
			ContextData: map[string]interface{}{
				"dish_name":     "beef wellington",
				"servings":      8,
				"complexity":    "advanced",
				"cuisine":       "french",
				"progress":      "completed",
				"key_techniques": []string{"pastry work", "duxelles", "temperature control"},
				"timing":        "preparation timeline provided",
			},
			CreatedAt: time.Now().Add(-2 * time.Hour),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		},
		{
			ConversationID: conversationID,
			ContextType:    "user_preferences",
			ContextData: map[string]interface{}{
				"skill_level":      "intermediate",
				"pastry_experience": true,
				"pate_experience":   false,
				"beef_grade":       "prime",
				"special_occasion": true,
			},
			CreatedAt: time.Now().Add(-110 * time.Minute),
			UpdatedAt: time.Now().Add(-90 * time.Minute),
		},
		{
			ConversationID: conversationID,
			ContextType:    "ai_metadata",
			ContextData: map[string]interface{}{
				"total_messages":    20,
				"total_tokens":      sum(extractTokens(messages)),
				"avg_response_time": 175,
				"provider":         "test",
				"conversation_quality": "high",
			},
			CreatedAt: time.Now().Add(-30 * time.Minute),
			UpdatedAt: time.Now().Add(-30 * time.Minute),
		},
	}
	
	return &ConversationFixture{
		Conversation: conv,
		Messages:     messages[:len(messageContents)],
		Context:      contexts,
	}
}

// GetPredefinedScenarios returns predefined conversation scenarios for testing
func GetPredefinedScenarios() []ConversationScenario {
	return []ConversationScenario{
		{
			Name:   "Quick Pasta Recipe",
			UserID: "scenario-user-1",
			Intent: conversation.IntentRecipeCreation,
			MessageFlow: []ScenarioMessageFlow{
				{
					UserMessage:      "I need a quick pasta recipe for tonight",
					ExpectedResponse: "pasta",
					ContextUpdates: map[string]interface{}{
						"urgency": "tonight",
						"dish_type": "pasta",
					},
				},
				{
					UserMessage:      "Something with tomatoes, for 2 people",
					ExpectedResponse: "tomato",
					ContextUpdates: map[string]interface{}{
						"ingredients": []string{"tomatoes"},
						"servings": 2,
					},
				},
				{
					UserMessage:      "Yes, create the recipe",
					ExpectedResponse: "recipe",
					ContextUpdates: map[string]interface{}{
						"status": "completed",
					},
				},
			},
			ExpectedFlow: []string{"gathering_info", "gathering_details", "creating_recipe"},
		},
		{
			Name:   "Cooking Technique Help",
			UserID: "scenario-user-2",
			Intent: conversation.IntentCookingHelp,
			MessageFlow: []ScenarioMessageFlow{
				{
					UserMessage:      "How do I cook rice perfectly?",
					ExpectedResponse: "rice",
					ContextUpdates: map[string]interface{}{
						"technique": "rice_cooking",
					},
				},
				{
					UserMessage:      "What's the water ratio?",
					ExpectedResponse: "ratio",
					ContextUpdates: map[string]interface{}{
						"specific_question": "water_ratio",
					},
				},
			},
			ExpectedFlow: []string{"providing_help", "detailed_explanation"},
		},
		{
			Name:   "Emergency Substitution",
			UserID: "scenario-user-3",
			Intent: conversation.IntentIngredientSubst,
			MessageFlow: []ScenarioMessageFlow{
				{
					UserMessage:      "I'm out of eggs while baking a cake!",
					ExpectedResponse: "substitute",
					ContextUpdates: map[string]interface{}{
						"urgency": "immediate",
						"missing_ingredient": "eggs",
						"recipe_type": "cake",
					},
				},
				{
					UserMessage:      "I have applesauce and baking powder",
					ExpectedResponse: "applesauce",
					ContextUpdates: map[string]interface{}{
						"available_substitutes": []string{"applesauce", "baking powder"},
					},
				},
			},
			ExpectedFlow: []string{"emergency_substitution", "solution_provided"},
		},
	}
}

// Helper functions

func extractTokens(messages []*conversation.Message) []int {
	tokens := make([]int, len(messages))
	for i, msg := range messages {
		if msg != nil {
			tokens[i] = msg.TokensUsed
		}
	}
	return tokens
}

func sum(numbers []int) int {
	total := 0
	for _, n := range numbers {
		total += n
	}
	return total
}

// ConversationFixtureBuilder provides a builder pattern for creating custom fixtures
type ConversationFixtureBuilder struct {
	conversation *conversation.Conversation
	messages     []*conversation.Message
	contexts     []*conversation.ConversationContext
}

// NewConversationFixtureBuilder creates a new fixture builder
func NewConversationFixtureBuilder() *ConversationFixtureBuilder {
	return &ConversationFixtureBuilder{
		messages: make([]*conversation.Message, 0),
		contexts: make([]*conversation.ConversationContext, 0),
	}
}

// WithConversation sets the conversation for the fixture
func (b *ConversationFixtureBuilder) WithConversation(userID string, intent conversation.ConversationIntent, title string) *ConversationFixtureBuilder {
	b.conversation = &conversation.Conversation{
		ID:     uuid.New().String(),
		UserID: userID,
		Title:  title,
		Intent: intent,
		Status: conversation.StatusActive,
		Metadata: make(map[string]interface{}),
		CreatedAt: time.Now().Add(-1 * time.Hour),
		UpdatedAt: time.Now().Add(-10 * time.Minute),
	}
	return b
}

// AddMessage adds a message to the fixture
func (b *ConversationFixtureBuilder) AddMessage(role conversation.MessageRole, content string) *ConversationFixtureBuilder {
	if b.conversation == nil {
		panic("Conversation must be set before adding messages")
	}
	
	message := &conversation.Message{
		ID:             uuid.New().String(),
		ConversationID: b.conversation.ID,
		Role:           role,
		Content:        content,
		Metadata:       make(map[string]interface{}),
		TokensUsed:     len(strings.Fields(content)),
		CreatedAt:      time.Now().Add(-time.Duration(len(b.messages)*5) * time.Minute),
	}
	
	if role == conversation.RoleAssistant {
		message.Metadata = map[string]interface{}{
			"ai_provider": "test",
			"confidence":  0.9,
		}
		message.ProcessingTimeMs = 150
		message.ModelUsed = "test-model"
	}
	
	b.messages = append(b.messages, message)
	return b
}

// AddContext adds context to the fixture
func (b *ConversationFixtureBuilder) AddContext(contextType string, data map[string]interface{}) *ConversationFixtureBuilder {
	if b.conversation == nil {
		panic("Conversation must be set before adding context")
	}
	
	context := &conversation.ConversationContext{
		ConversationID: b.conversation.ID,
		ContextType:    contextType,
		ContextData:    data,
		CreatedAt:      time.Now().Add(-30 * time.Minute),
		UpdatedAt:      time.Now().Add(-5 * time.Minute),
	}
	
	b.contexts = append(b.contexts, context)
	return b
}

// Build creates the conversation fixture
func (b *ConversationFixtureBuilder) Build() *ConversationFixture {
	if b.conversation == nil {
		panic("Conversation must be set")
	}
	
	return &ConversationFixture{
		Conversation: b.conversation,
		Messages:     b.messages,
		Context:      b.contexts,
	}
}

// ToJSON converts fixture to JSON for persistence or comparison
func (f *ConversationFixture) ToJSON() ([]byte, error) {
	return json.MarshalIndent(f, "", "  ")
}

// FromJSON creates a fixture from JSON data
func FromJSON(data []byte) (*ConversationFixture, error) {
	var fixture ConversationFixture
	err := json.Unmarshal(data, &fixture)
	if err != nil {
		return nil, err
	}
	return &fixture, nil
}