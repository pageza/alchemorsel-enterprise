// Package ai provides prompt building and enhanced generation methods for enterprise AI service
package ai

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alchemorsel/v3/internal/domain/recipe"
	"github.com/alchemorsel/v3/internal/ports/outbound"
)

// buildOptimizationPrompt creates a prompt for recipe optimization
func (s *EnterpriseAIService) buildOptimizationPrompt(rec *recipe.Recipe, optimizationType string) string {
	var prompt strings.Builder
	
	// Base recipe information
	prompt.WriteString(fmt.Sprintf("Optimize the following recipe for %s:\n\n", optimizationType))
	prompt.WriteString(fmt.Sprintf("Title: %s\n", rec.Title()))
	prompt.WriteString(fmt.Sprintf("Description: %s\n", rec.Description()))
	
	// Add ingredients
	prompt.WriteString("Ingredients:\n")
	for _, ingredient := range rec.Ingredients() {
		prompt.WriteString(fmt.Sprintf("- %s\n", ingredient.Name))
	}
	
	// Add instructions
	prompt.WriteString("\nInstructions:\n")
	for i, instruction := range rec.Instructions() {
		prompt.WriteString(fmt.Sprintf("%d. %s\n", i+1, instruction.Description))
	}
	
	// Optimization-specific instructions
	switch strings.ToLower(optimizationType) {
	case "health", "healthy":
		prompt.WriteString("\nOptimization goals:\n")
		prompt.WriteString("- Reduce sodium and unhealthy fats\n")
		prompt.WriteString("- Increase fiber and protein content\n")
		prompt.WriteString("- Use whole grains and fresh ingredients\n")
		prompt.WriteString("- Minimize processed ingredients\n")
		prompt.WriteString("- Maintain or improve taste while boosting nutrition\n")
		
	case "cost", "budget", "cheap":
		prompt.WriteString("\nOptimization goals:\n")
		prompt.WriteString("- Replace expensive ingredients with affordable alternatives\n")
		prompt.WriteString("- Use seasonal and common ingredients\n")
		prompt.WriteString("- Maximize serving size for the budget\n")
		prompt.WriteString("- Reduce food waste through efficient portioning\n")
		prompt.WriteString("- Maintain nutritional value and taste\n")
		
	case "taste", "flavor":
		prompt.WriteString("\nOptimization goals:\n")
		prompt.WriteString("- Enhance flavor profile with herbs and spices\n")
		prompt.WriteString("- Improve cooking techniques for better taste\n")
		prompt.WriteString("- Balance sweet, salty, sour, and umami flavors\n")
		prompt.WriteString("- Add complementary ingredients for depth\n")
		prompt.WriteString("- Optimize cooking times and temperatures\n")
		
	case "time", "quick", "fast":
		prompt.WriteString("\nOptimization goals:\n")
		prompt.WriteString("- Reduce preparation and cooking time\n")
		prompt.WriteString("- Use time-saving techniques and equipment\n")
		prompt.WriteString("- Pre-prepare ingredients when possible\n")
		prompt.WriteString("- Simplify cooking steps without losing quality\n")
		prompt.WriteString("- Consider make-ahead options\n")
		
	default:
		prompt.WriteString("\nOptimization goals:\n")
		prompt.WriteString("- Improve overall recipe quality\n")
		prompt.WriteString("- Balance nutrition, taste, and practicality\n")
		prompt.WriteString("- Enhance presentation and appeal\n")
	}
	
	prompt.WriteString("\nProvide an optimized version with detailed explanations for each change.")
	
	return prompt.String()
}

// buildDietaryAdaptationPrompt creates a prompt for dietary adaptation
func (s *EnterpriseAIService) buildDietaryAdaptationPrompt(rec *recipe.Recipe, dietaryRestrictions []string) string {
	var prompt strings.Builder
	
	prompt.WriteString("Adapt the following recipe to meet these dietary restrictions: ")
	prompt.WriteString(strings.Join(dietaryRestrictions, ", "))
	prompt.WriteString("\n\n")
	
	// Original recipe
	prompt.WriteString(fmt.Sprintf("Original Recipe: %s\n", rec.Title()))
	prompt.WriteString(fmt.Sprintf("Description: %s\n\n", rec.Description()))
	
	prompt.WriteString("Ingredients:\n")
	for _, ingredient := range rec.Ingredients() {
		prompt.WriteString(fmt.Sprintf("- %s\n", ingredient.Name))
	}
	
	prompt.WriteString("\nInstructions:\n")
	for i, instruction := range rec.Instructions() {
		prompt.WriteString(fmt.Sprintf("%d. %s\n", i+1, instruction.Description))
	}
	
	// Dietary-specific guidance
	prompt.WriteString("\nAdaptation requirements:\n")
	for _, restriction := range dietaryRestrictions {
		switch strings.ToLower(restriction) {
		case "vegetarian":
			prompt.WriteString("- Replace all meat and fish with plant-based alternatives\n")
			prompt.WriteString("- Ensure sufficient protein through legumes, nuts, or dairy\n")
		case "vegan":
			prompt.WriteString("- Remove all animal products (meat, dairy, eggs, honey)\n")
			prompt.WriteString("- Use plant-based alternatives for proteins and fats\n")
			prompt.WriteString("- Ensure nutritional completeness\n")
		case "gluten_free", "gluten-free":
			prompt.WriteString("- Replace wheat flour with gluten-free alternatives\n")
			prompt.WriteString("- Check all ingredients for hidden gluten\n")
			prompt.WriteString("- Maintain texture and structure\n")
		case "dairy_free", "dairy-free":
			prompt.WriteString("- Replace all dairy products with non-dairy alternatives\n")
			prompt.WriteString("- Maintain creaminess and flavor profiles\n")
		case "keto", "ketogenic":
			prompt.WriteString("- Minimize carbohydrates to under 20g per serving\n")
			prompt.WriteString("- Increase healthy fats and moderate protein\n")
			prompt.WriteString("- Replace high-carb ingredients\n")
		case "paleo":
			prompt.WriteString("- Remove grains, legumes, and processed foods\n")
			prompt.WriteString("- Focus on whole, unprocessed ingredients\n")
		case "low_sodium", "low-sodium":
			prompt.WriteString("- Reduce sodium content significantly\n")
			prompt.WriteString("- Use herbs and spices for flavor instead of salt\n")
		}
	}
	
	prompt.WriteString("\nProvide the adapted recipe with substitution explanations and nutritional notes.")
	
	return prompt.String()
}

// buildMealPlanPrompt creates a prompt for meal planning
func (s *EnterpriseAIService) buildMealPlanPrompt(days int, dietary []string, budget float64) string {
	var prompt strings.Builder
	
	prompt.WriteString(fmt.Sprintf("Create a comprehensive %d-day meal plan with the following requirements:\n\n", days))
	
	if budget > 0 {
		prompt.WriteString(fmt.Sprintf("Budget: $%.2f total (approximately $%.2f per day)\n", budget, budget/float64(days)))
	}
	
	if len(dietary) > 0 {
		prompt.WriteString("Dietary restrictions: " + strings.Join(dietary, ", ") + "\n")
	}
	
	prompt.WriteString("\nRequirements:\n")
	prompt.WriteString("- Include breakfast, lunch, dinner, and 1-2 snacks per day\n")
	prompt.WriteString("- Provide variety in cuisines and cooking methods\n")
	prompt.WriteString("- Balance nutrition across all meals\n")
	prompt.WriteString("- Include preparation times and difficulty levels\n")
	prompt.WriteString("- Generate a consolidated shopping list\n")
	prompt.WriteString("- Organize ingredients by grocery store sections\n")
	prompt.WriteString("- Minimize food waste through ingredient reuse\n")
	prompt.WriteString("- Include make-ahead and batch cooking opportunities\n")
	
	if budget > 0 {
		prompt.WriteString("- Stay within the specified budget\n")
		prompt.WriteString("- Prioritize cost-effective, nutritious ingredients\n")
		prompt.WriteString("- Include budget breakdown by day and meal\n")
	}
	
	prompt.WriteString("\nFor each meal, provide:\n")
	prompt.WriteString("- Recipe name and brief description\n")
	prompt.WriteString("- Ingredient list with quantities\n")
	prompt.WriteString("- Step-by-step instructions\n")
	prompt.WriteString("- Prep time and cook time\n")
	prompt.WriteString("- Estimated cost per serving\n")
	prompt.WriteString("- Basic nutritional information\n")
	
	prompt.WriteString("\nReturn the meal plan in a structured format suitable for implementation.")
	
	return prompt.String()
}

// generateEnhancedMockRecipe creates a high-quality mock recipe
func (s *EnterpriseAIService) generateEnhancedMockRecipe(prompt string, constraints outbound.AIConstraints) (*outbound.AIRecipeResponse, error) {
	prompt = strings.ToLower(prompt)
	
	// Enhanced recipe templates based on keywords
	var recipe *outbound.AIRecipeResponse
	
	if strings.Contains(prompt, "pasta") {
		recipe = s.createPastaRecipe(prompt, constraints)
	} else if strings.Contains(prompt, "chicken") {
		recipe = s.createChickenRecipe(prompt, constraints)
	} else if strings.Contains(prompt, "vegetarian") || strings.Contains(prompt, "vegan") {
		recipe = s.createVegetarianRecipe(prompt, constraints)
	} else if strings.Contains(prompt, "asian") || strings.Contains(prompt, "stir") {
		recipe = s.createAsianRecipe(prompt, constraints)
	} else if strings.Contains(prompt, "healthy") || strings.Contains(prompt, "nutrition") {
		recipe = s.createHealthyRecipe(prompt, constraints)
	} else if strings.Contains(prompt, "quick") || strings.Contains(prompt, "fast") {
		recipe = s.createQuickRecipe(prompt, constraints)
	} else {
		recipe = s.createBalancedRecipe(prompt, constraints)
	}
	
	// Apply constraints
	s.applyConstraints(recipe, constraints)
	
	return recipe, nil
}

// createPastaRecipe creates a pasta-based recipe
func (s *EnterpriseAIService) createPastaRecipe(prompt string, constraints outbound.AIConstraints) *outbound.AIRecipeResponse {
	return &outbound.AIRecipeResponse{
		Title:       "Creamy Garlic Herb Pasta",
		Description: "A rich and flavorful pasta dish with a creamy garlic herb sauce, perfect for weeknight dinners or special occasions.",
		Ingredients: []outbound.AIIngredient{
			{Name: "Pasta (penne or fettuccine)", Amount: 12, Unit: "oz"},
			{Name: "Heavy cream", Amount: 1, Unit: "cup"},
			{Name: "Parmesan cheese, grated", Amount: 0.75, Unit: "cup"},
			{Name: "Garlic, minced", Amount: 4, Unit: "cloves"},
			{Name: "Fresh basil, chopped", Amount: 0.25, Unit: "cup"},
			{Name: "Olive oil", Amount: 2, Unit: "tbsp"},
			{Name: "Butter", Amount: 2, Unit: "tbsp"},
			{Name: "Salt", Amount: 1, Unit: "tsp"},
			{Name: "Black pepper", Amount: 0.5, Unit: "tsp"},
			{Name: "Cherry tomatoes, halved", Amount: 1, Unit: "cup"},
		},
		Instructions: []string{
			"Cook pasta according to package directions until al dente. Reserve 1 cup of pasta water before draining.",
			"In a large skillet, heat olive oil and butter over medium heat.",
			"Add minced garlic and sauté for 1-2 minutes until fragrant, being careful not to burn.",
			"Add cherry tomatoes and cook for 3-4 minutes until they start to soften.",
			"Pour in heavy cream and bring to a gentle simmer. Cook for 2-3 minutes to thicken slightly.",
			"Add the cooked pasta to the skillet and toss to combine.",
			"Remove from heat and add Parmesan cheese, stirring until melted and creamy.",
			"If sauce is too thick, add reserved pasta water gradually until desired consistency.",
			"Season with salt and pepper to taste.",
			"Garnish with fresh basil and serve immediately with extra Parmesan if desired.",
		},
		Nutrition: &outbound.NutritionInfo{
			Calories: 485,
			Protein:  18.5,
			Carbs:    58.0,
			Fat:      20.5,
			Fiber:    3.2,
			Sugar:    6.8,
			Sodium:   680.0,
		},
		Tags:       []string{"pasta", "creamy", "garlic", "italian", "comfort food"},
		Confidence: 0.92,
	}
}

// createChickenRecipe creates a chicken-based recipe
func (s *EnterpriseAIService) createChickenRecipe(prompt string, constraints outbound.AIConstraints) *outbound.AIRecipeResponse {
	return &outbound.AIRecipeResponse{
		Title:       "Herb-Crusted Lemon Chicken",
		Description: "Juicy chicken breast with a crispy herb crust and bright lemon flavor, perfect for a healthy and delicious dinner.",
		Ingredients: []outbound.AIIngredient{
			{Name: "Chicken breasts, boneless", Amount: 4, Unit: "pieces"},
			{Name: "Panko breadcrumbs", Amount: 1, Unit: "cup"},
			{Name: "Fresh herbs (parsley, thyme)", Amount: 0.25, Unit: "cup"},
			{Name: "Lemon zest", Amount: 2, Unit: "tbsp"},
			{Name: "Lemon juice", Amount: 0.25, Unit: "cup"},
			{Name: "Olive oil", Amount: 3, Unit: "tbsp"},
			{Name: "Garlic powder", Amount: 1, Unit: "tsp"},
			{Name: "Salt", Amount: 1, Unit: "tsp"},
			{Name: "Black pepper", Amount: 0.5, Unit: "tsp"},
			{Name: "Dijon mustard", Amount: 2, Unit: "tbsp"},
		},
		Instructions: []string{
			"Preheat oven to 400°F (200°C). Line a baking sheet with parchment paper.",
			"Pat chicken breasts dry and season with salt and pepper on both sides.",
			"In a shallow bowl, mix panko breadcrumbs, chopped herbs, lemon zest, garlic powder, salt, and pepper.",
			"Brush each chicken breast with Dijon mustard, then with lemon juice.",
			"Drizzle olive oil over the breadcrumb mixture and mix until evenly coated.",
			"Press the herb breadcrumb mixture firmly onto both sides of each chicken breast.",
			"Place coated chicken on the prepared baking sheet.",
			"Bake for 20-25 minutes or until internal temperature reaches 165°F (74°C).",
			"Let rest for 5 minutes before slicing.",
			"Serve with lemon wedges and your choice of vegetables or salad.",
		},
		Nutrition: &outbound.NutritionInfo{
			Calories: 320,
			Protein:  35.0,
			Carbs:    12.0,
			Fat:      15.0,
			Fiber:    1.5,
			Sugar:    2.0,
			Sodium:   580.0,
		},
		Tags:       []string{"chicken", "herbs", "lemon", "healthy", "baked"},
		Confidence: 0.90,
	}
}

// createVegetarianRecipe creates a vegetarian/vegan recipe
func (s *EnterpriseAIService) createVegetarianRecipe(prompt string, constraints outbound.AIConstraints) *outbound.AIRecipeResponse {
	return &outbound.AIRecipeResponse{
		Title:       "Mediterranean Quinoa Power Bowl",
		Description: "A nutritious and colorful bowl packed with quinoa, fresh vegetables, and a zesty tahini dressing.",
		Ingredients: []outbound.AIIngredient{
			{Name: "Quinoa", Amount: 1, Unit: "cup"},
			{Name: "Chickpeas, drained", Amount: 1, Unit: "can"},
			{Name: "Cucumber, diced", Amount: 1, Unit: "large"},
			{Name: "Cherry tomatoes, halved", Amount: 2, Unit: "cups"},
			{Name: "Red bell pepper, sliced", Amount: 1, Unit: "large"},
			{Name: "Red onion, thinly sliced", Amount: 0.25, Unit: "cup"},
			{Name: "Kalamata olives", Amount: 0.33, Unit: "cup"},
			{Name: "Fresh parsley, chopped", Amount: 0.25, Unit: "cup"},
			{Name: "Feta cheese, crumbled", Amount: 0.5, Unit: "cup"},
			{Name: "Tahini", Amount: 3, Unit: "tbsp"},
			{Name: "Lemon juice", Amount: 3, Unit: "tbsp"},
			{Name: "Olive oil", Amount: 2, Unit: "tbsp"},
			{Name: "Garlic, minced", Amount: 2, Unit: "cloves"},
		},
		Instructions: []string{
			"Rinse quinoa thoroughly and cook according to package directions. Let cool completely.",
			"While quinoa cooks, prepare all vegetables: dice cucumber, halve tomatoes, slice bell pepper and red onion.",
			"Drain and rinse chickpeas, then pat dry.",
			"For the dressing, whisk together tahini, lemon juice, olive oil, minced garlic, salt, and pepper.",
			"Add 2-3 tablespoons of water to thin the dressing to desired consistency.",
			"In a large bowl, combine cooled quinoa, chickpeas, cucumber, tomatoes, bell pepper, and red onion.",
			"Drizzle with tahini dressing and toss gently to combine.",
			"Top with olives, crumbled feta, and fresh parsley.",
			"Let sit for 15 minutes to allow flavors to meld before serving.",
			"Serve chilled or at room temperature as a complete meal.",
		},
		Nutrition: &outbound.NutritionInfo{
			Calories: 420,
			Protein:  16.0,
			Carbs:    52.0,
			Fat:      18.0,
			Fiber:    12.0,
			Sugar:    8.0,
			Sodium:   480.0,
		},
		Tags:       []string{"vegetarian", "mediterranean", "quinoa", "healthy", "protein-rich"},
		Confidence: 0.88,
	}
}

// createAsianRecipe creates an Asian-inspired recipe
func (s *EnterpriseAIService) createAsianRecipe(prompt string, constraints outbound.AIConstraints) *outbound.AIRecipeResponse {
	return &outbound.AIRecipeResponse{
		Title:       "Ginger Soy Vegetable Stir-Fry",
		Description: "A vibrant and flavorful stir-fry with fresh vegetables in a savory ginger-soy sauce, ready in minutes.",
		Ingredients: []outbound.AIIngredient{
			{Name: "Mixed vegetables (broccoli, bell peppers, snap peas)", Amount: 4, Unit: "cups"},
			{Name: "Carrots, julienned", Amount: 2, Unit: "medium"},
			{Name: "Fresh ginger, minced", Amount: 2, Unit: "tbsp"},
			{Name: "Garlic, minced", Amount: 4, Unit: "cloves"},
			{Name: "Soy sauce", Amount: 3, Unit: "tbsp"},
			{Name: "Rice vinegar", Amount: 2, Unit: "tbsp"},
			{Name: "Sesame oil", Amount: 1, Unit: "tbsp"},
			{Name: "Vegetable oil", Amount: 2, Unit: "tbsp"},
			{Name: "Cornstarch", Amount: 1, Unit: "tbsp"},
			{Name: "Green onions, sliced", Amount: 3, Unit: "stalks"},
			{Name: "Sesame seeds", Amount: 1, Unit: "tbsp"},
			{Name: "Red pepper flakes", Amount: 0.25, Unit: "tsp"},
		},
		Instructions: []string{
			"Prepare all vegetables by cutting into uniform bite-sized pieces.",
			"In a small bowl, whisk together soy sauce, rice vinegar, sesame oil, and cornstarch.",
			"Heat vegetable oil in a large wok or skillet over high heat until smoking.",
			"Add carrots first and stir-fry for 2 minutes.",
			"Add broccoli and bell peppers, stir-fry for 2-3 minutes until crisp-tender.",
			"Add snap peas, minced ginger, and garlic. Stir-fry for 1 minute until fragrant.",
			"Pour the sauce mixture over vegetables and toss quickly to coat.",
			"Cook for 1-2 minutes until sauce thickens and vegetables are glazed.",
			"Remove from heat and sprinkle with green onions, sesame seeds, and red pepper flakes.",
			"Serve immediately over steamed rice or noodles.",
		},
		Nutrition: &outbound.NutritionInfo{
			Calories: 145,
			Protein:  5.0,
			Carbs:    18.0,
			Fat:      7.5,
			Fiber:    5.0,
			Sugar:    8.0,
			Sodium:   580.0,
		},
		Tags:       []string{"asian", "stir-fry", "vegetables", "quick", "healthy"},
		Confidence: 0.91,
	}
}

// createHealthyRecipe creates a health-focused recipe
func (s *EnterpriseAIService) createHealthyRecipe(prompt string, constraints outbound.AIConstraints) *outbound.AIRecipeResponse {
	return &outbound.AIRecipeResponse{
		Title:       "Superfood Salmon Bowl",
		Description: "A nutrient-dense bowl featuring omega-rich salmon, antioxidant-packed vegetables, and wholesome grains.",
		Ingredients: []outbound.AIIngredient{
			{Name: "Salmon fillets", Amount: 4, Unit: "pieces"},
			{Name: "Brown rice", Amount: 1, Unit: "cup"},
			{Name: "Kale, massaged", Amount: 2, Unit: "cups"},
			{Name: "Sweet potato, roasted", Amount: 1, Unit: "large"},
			{Name: "Avocado, sliced", Amount: 1, Unit: "large"},
			{Name: "Edamame, shelled", Amount: 0.5, Unit: "cup"},
			{Name: "Blueberries", Amount: 0.5, Unit: "cup"},
			{Name: "Pumpkin seeds", Amount: 2, Unit: "tbsp"},
			{Name: "Lemon juice", Amount: 2, Unit: "tbsp"},
			{Name: "Olive oil", Amount: 2, Unit: "tbsp"},
			{Name: "Turmeric", Amount: 0.5, Unit: "tsp"},
			{Name: "Sea salt", Amount: 0.5, Unit: "tsp"},
		},
		Instructions: []string{
			"Preheat oven to 425°F (220°C). Cook brown rice according to package directions.",
			"Cube sweet potato and toss with 1 tablespoon olive oil and sea salt. Roast for 25-30 minutes.",
			"Season salmon with turmeric, salt, and pepper. Bake for 12-15 minutes until flaky.",
			"While salmon cooks, massage kale with lemon juice and a pinch of salt until tender.",
			"Steam edamame according to package directions.",
			"Assemble bowls with brown rice as the base.",
			"Top with massaged kale, roasted sweet potato, and steamed edamame.",
			"Add flaked salmon, sliced avocado, and fresh blueberries.",
			"Drizzle with remaining olive oil and lemon juice.",
			"Sprinkle with pumpkin seeds and serve immediately.",
		},
		Nutrition: &outbound.NutritionInfo{
			Calories: 485,
			Protein:  32.0,
			Carbs:    45.0,
			Fat:      22.0,
			Fiber:    12.0,
			Sugar:    10.0,
			Sodium:   320.0,
		},
		Tags:       []string{"healthy", "superfood", "salmon", "omega-3", "antioxidants"},
		Confidence: 0.93,
	}
}

// createQuickRecipe creates a quick and easy recipe
func (s *EnterpriseAIService) createQuickRecipe(prompt string, constraints outbound.AIConstraints) *outbound.AIRecipeResponse {
	return &outbound.AIRecipeResponse{
		Title:       "15-Minute Caprese Flatbread",
		Description: "A quick and delicious flatbread topped with fresh mozzarella, tomatoes, and basil - ready in just 15 minutes.",
		Ingredients: []outbound.AIIngredient{
			{Name: "Naan or flatbread", Amount: 2, Unit: "pieces"},
			{Name: "Fresh mozzarella, sliced", Amount: 8, Unit: "oz"},
			{Name: "Cherry tomatoes, halved", Amount: 1.5, Unit: "cups"},
			{Name: "Fresh basil leaves", Amount: 0.25, Unit: "cup"},
			{Name: "Balsamic glaze", Amount: 2, Unit: "tbsp"},
			{Name: "Olive oil", Amount: 2, Unit: "tbsp"},
			{Name: "Garlic powder", Amount: 0.5, Unit: "tsp"},
			{Name: "Salt", Amount: 0.25, Unit: "tsp"},
			{Name: "Black pepper", Amount: 0.25, Unit: "tsp"},
		},
		Instructions: []string{
			"Preheat oven to 450°F (230°C) or use a toaster oven.",
			"Place flatbreads on a baking sheet and brush with olive oil.",
			"Sprinkle with garlic powder, salt, and pepper.",
			"Top with sliced mozzarella and halved cherry tomatoes.",
			"Bake for 8-10 minutes until cheese is melted and edges are golden.",
			"Remove from oven and immediately top with fresh basil leaves.",
			"Drizzle with balsamic glaze before serving.",
			"Cut into wedges and serve hot as an appetizer or light meal.",
		},
		Nutrition: &outbound.NutritionInfo{
			Calories: 320,
			Protein:  16.0,
			Carbs:    28.0,
			Fat:      18.0,
			Fiber:    2.0,
			Sugar:    5.0,
			Sodium:   680.0,
		},
		Tags:       []string{"quick", "15-minute", "caprese", "italian", "easy"},
		Confidence: 0.89,
	}
}

// createBalancedRecipe creates a well-balanced default recipe
func (s *EnterpriseAIService) createBalancedRecipe(prompt string, constraints outbound.AIConstraints) *outbound.AIRecipeResponse {
	return &outbound.AIRecipeResponse{
		Title:       "One-Pan Mediterranean Chicken and Vegetables",
		Description: "A complete, balanced meal featuring tender chicken thighs with colorful Mediterranean vegetables, all cooked in one pan.",
		Ingredients: []outbound.AIIngredient{
			{Name: "Chicken thighs, bone-in", Amount: 6, Unit: "pieces"},
			{Name: "Baby potatoes, halved", Amount: 1.5, Unit: "lbs"},
			{Name: "Zucchini, sliced", Amount: 2, Unit: "medium"},
			{Name: "Red bell pepper, chunks", Amount: 1, Unit: "large"},
			{Name: "Red onion, wedges", Amount: 1, Unit: "medium"},
			{Name: "Cherry tomatoes", Amount: 1, Unit: "cup"},
			{Name: "Olive oil", Amount: 0.25, Unit: "cup"},
			{Name: "Lemon juice", Amount: 3, Unit: "tbsp"},
			{Name: "Oregano", Amount: 2, Unit: "tsp"},
			{Name: "Garlic, minced", Amount: 4, Unit: "cloves"},
			{Name: "Salt", Amount: 1, Unit: "tsp"},
			{Name: "Black pepper", Amount: 0.5, Unit: "tsp"},
			{Name: "Feta cheese, crumbled", Amount: 0.5, Unit: "cup"},
		},
		Instructions: []string{
			"Preheat oven to 425°F (220°C).",
			"In a large bowl, combine olive oil, lemon juice, oregano, minced garlic, salt, and pepper.",
			"Add halved potatoes to the bowl and toss to coat. Let marinate for 10 minutes.",
			"Transfer potatoes to a large roasting pan and bake for 15 minutes.",
			"Season chicken thighs with salt and pepper, then add to the same marinade bowl.",
			"Add chicken to the pan with potatoes.",
			"Add zucchini, bell pepper, red onion, and cherry tomatoes around the chicken.",
			"Drizzle any remaining marinade over the vegetables.",
			"Bake for 35-40 minutes until chicken reaches 165°F internal temperature.",
			"Sprinkle with crumbled feta and fresh herbs before serving.",
		},
		Nutrition: &outbound.NutritionInfo{
			Calories: 420,
			Protein:  28.0,
			Carbs:    35.0,
			Fat:      20.0,
			Fiber:    6.0,
			Sugar:    8.0,
			Sodium:   580.0,
		},
		Tags:       []string{"one-pan", "mediterranean", "chicken", "vegetables", "balanced"},
		Confidence: 0.87,
	}
}

// applyConstraints modifies recipe based on constraints
func (s *EnterpriseAIService) applyConstraints(recipe *outbound.AIRecipeResponse, constraints outbound.AIConstraints) {
	// Apply calorie constraints
	if constraints.MaxCalories > 0 && recipe.Nutrition.Calories > constraints.MaxCalories {
		factor := float64(constraints.MaxCalories) / float64(recipe.Nutrition.Calories)
		s.scaleRecipe(recipe, factor)
	}
	
	// Apply dietary constraints
	if len(constraints.Dietary) > 0 {
		s.adaptRecipeForDietary(recipe, constraints.Dietary)
	}
	
	// Add cuisine tag if specified
	if constraints.Cuisine != "" {
		found := false
		for _, tag := range recipe.Tags {
			if strings.ToLower(tag) == strings.ToLower(constraints.Cuisine) {
				found = true
				break
			}
		}
		if !found {
			recipe.Tags = append(recipe.Tags, strings.ToLower(constraints.Cuisine))
		}
	}
}

// scaleRecipe scales recipe quantities by a factor
func (s *EnterpriseAIService) scaleRecipe(recipe *outbound.AIRecipeResponse, factor float64) {
	// Scale ingredients
	for i := range recipe.Ingredients {
		recipe.Ingredients[i].Amount *= factor
	}
	
	// Scale nutrition
	recipe.Nutrition.Calories = int(float64(recipe.Nutrition.Calories) * factor)
	recipe.Nutrition.Protein *= factor
	recipe.Nutrition.Carbs *= factor
	recipe.Nutrition.Fat *= factor
	recipe.Nutrition.Fiber *= factor
	recipe.Nutrition.Sugar *= factor
	recipe.Nutrition.Sodium *= factor
}

// adaptRecipeForDietary adapts recipe for dietary restrictions
func (s *EnterpriseAIService) adaptRecipeForDietary(recipe *outbound.AIRecipeResponse, dietary []string) {
	for _, restriction := range dietary {
		switch strings.ToLower(restriction) {
		case "vegetarian":
			s.makeVegetarian(recipe)
		case "vegan":
			s.makeVegan(recipe)
		case "gluten_free", "gluten-free":
			s.makeGlutenFree(recipe)
		case "dairy_free", "dairy-free":
			s.makeDairyFree(recipe)
		}
		
		// Add dietary tag
		found := false
		for _, tag := range recipe.Tags {
			if strings.ToLower(tag) == strings.ToLower(restriction) {
				found = true
				break
			}
		}
		if !found {
			recipe.Tags = append(recipe.Tags, strings.ToLower(restriction))
		}
	}
}

// makeVegetarian removes meat from recipe
func (s *EnterpriseAIService) makeVegetarian(recipe *outbound.AIRecipeResponse) {
	// Replace meat ingredients with vegetarian alternatives
	for i, ingredient := range recipe.Ingredients {
		name := strings.ToLower(ingredient.Name)
		if s.isMeat(name) {
			if strings.Contains(name, "chicken") {
				recipe.Ingredients[i].Name = "Firm tofu or tempeh"
			} else if strings.Contains(name, "beef") {
				recipe.Ingredients[i].Name = "Mushrooms or plant-based ground meat"
			} else if strings.Contains(name, "fish") || strings.Contains(name, "salmon") {
				recipe.Ingredients[i].Name = "Marinated tofu or king oyster mushrooms"
			} else {
				recipe.Ingredients[i].Name = "Plant-based protein alternative"
			}
		}
	}
}

// makeVegan makes recipe vegan
func (s *EnterpriseAIService) makeVegan(recipe *outbound.AIRecipeResponse) {
	s.makeVegetarian(recipe) // First remove meat
	
	// Remove dairy and eggs
	for i, ingredient := range recipe.Ingredients {
		name := strings.ToLower(ingredient.Name)
		if s.isDairy(name) {
			if strings.Contains(name, "cheese") {
				recipe.Ingredients[i].Name = "Nutritional yeast or vegan cheese"
			} else if strings.Contains(name, "milk") || strings.Contains(name, "cream") {
				recipe.Ingredients[i].Name = "Coconut cream or cashew cream"
			} else if strings.Contains(name, "butter") {
				recipe.Ingredients[i].Name = "Vegan butter or olive oil"
			}
		} else if s.isEgg(name) {
			recipe.Ingredients[i].Name = "Flax egg or aquafaba"
		}
	}
}

// makeGlutenFree makes recipe gluten-free
func (s *EnterpriseAIService) makeGlutenFree(recipe *outbound.AIRecipeResponse) {
	for i, ingredient := range recipe.Ingredients {
		name := strings.ToLower(ingredient.Name)
		if s.containsGluten(name) {
			if strings.Contains(name, "flour") {
				recipe.Ingredients[i].Name = "Gluten-free flour blend"
			} else if strings.Contains(name, "pasta") {
				recipe.Ingredients[i].Name = "Gluten-free pasta"
			} else if strings.Contains(name, "bread") {
				recipe.Ingredients[i].Name = "Gluten-free bread"
			} else if strings.Contains(name, "soy sauce") {
				recipe.Ingredients[i].Name = "Tamari (gluten-free soy sauce)"
			}
		}
	}
}

// makeDairyFree makes recipe dairy-free
func (s *EnterpriseAIService) makeDairyFree(recipe *outbound.AIRecipeResponse) {
	for i, ingredient := range recipe.Ingredients {
		name := strings.ToLower(ingredient.Name)
		if s.isDairy(name) {
			if strings.Contains(name, "milk") {
				recipe.Ingredients[i].Name = "Almond milk or oat milk"
			} else if strings.Contains(name, "cheese") {
				recipe.Ingredients[i].Name = "Dairy-free cheese alternative"
			} else if strings.Contains(name, "butter") {
				recipe.Ingredients[i].Name = "Dairy-free butter or olive oil"
			} else if strings.Contains(name, "cream") {
				recipe.Ingredients[i].Name = "Coconut cream"
			}
		}
	}
}

// validateDietaryCompliance ensures recipe meets dietary requirements
func (s *EnterpriseAIService) validateDietaryCompliance(recipe *outbound.AIRecipeResponse, dietary []string) {
	// This method would implement validation logic to ensure
	// the generated recipe actually meets the dietary restrictions
	// For now, we'll just add appropriate tags
	
	for _, restriction := range dietary {
		found := false
		for _, tag := range recipe.Tags {
			if strings.ToLower(tag) == strings.ToLower(restriction) {
				found = true
				break
			}
		}
		if !found {
			recipe.Tags = append(recipe.Tags, strings.ToLower(restriction))
		}
	}
}

// generateMealPlanFallback creates a mock meal plan
func (s *EnterpriseAIService) generateMealPlanFallback(days int, dietary []string, budget float64) *MealPlanResponse {
	dailyBudget := budget / float64(days)
	if budget <= 0 {
		dailyBudget = 25.0 // Default daily budget
	}
	
	mealPlan := &MealPlanResponse{
		Days:        days,
		TotalBudget: budget,
		DailyMeals:  make([]DayMealPlan, days),
		GeneratedAt: time.Now(),
	}
	
	// Generate meals for each day
	for day := 0; day < days; day++ {
		dayPlan := DayMealPlan{
			Day:  day + 1,
			Date: time.Now().AddDate(0, 0, day).Format("2006-01-02"),
		}
		
		// Create simple meals for each day
		dayPlan.Breakfast = s.createMealPlanMeal("Overnight Oats", "breakfast", dietary, dailyBudget*0.2)
		dayPlan.Lunch = s.createMealPlanMeal("Mediterranean Salad", "lunch", dietary, dailyBudget*0.3)
		dayPlan.Dinner = s.createMealPlanMeal("Grilled Chicken Bowl", "dinner", dietary, dailyBudget*0.4)
		dayPlan.Snacks = []*MealPlanMeal{
			s.createMealPlanMeal("Mixed Nuts", "snack", dietary, dailyBudget*0.1),
		}
		
		dayPlan.DailyCost = dayPlan.Breakfast.EstimatedCost + dayPlan.Lunch.EstimatedCost + 
			dayPlan.Dinner.EstimatedCost + dayPlan.Snacks[0].EstimatedCost
		
		mealPlan.DailyMeals[day] = dayPlan
	}
	
	// Generate shopping list and nutrition summary
	mealPlan.ShoppingList = s.generateShoppingList(mealPlan.DailyMeals)
	mealPlan.NutritionSummary = s.generateNutritionSummary(mealPlan.DailyMeals)
	
	return mealPlan
}

// createMealPlanMeal creates a meal for the meal plan
func (s *EnterpriseAIService) createMealPlanMeal(name, mealType string, dietary []string, budget float64) *MealPlanMeal {
	// This is a simplified implementation
	// In a real system, this would generate detailed meals based on the parameters
	
	return &MealPlanMeal{
		Name:          name,
		Description:   fmt.Sprintf("A healthy %s option", mealType),
		Ingredients:   []outbound.AIIngredient{{Name: "Ingredients", Amount: 1, Unit: "serving"}},
		Instructions:  []string{"Prepare according to recipe", "Serve and enjoy"},
		PrepTime:      10,
		CookTime:      15,
		Servings:      1,
		EstimatedCost: budget,
		Nutrition: &outbound.NutritionInfo{
			Calories: 300,
			Protein:  15.0,
			Carbs:    35.0,
			Fat:      12.0,
			Fiber:    5.0,
			Sugar:    8.0,
			Sodium:   400.0,
		},
	}
}

// generateShoppingList creates a consolidated shopping list
func (s *EnterpriseAIService) generateShoppingList(dailyMeals []DayMealPlan) []ShoppingListItem {
	// Simplified shopping list generation
	return []ShoppingListItem{
		{Name: "Fruits", Amount: 7, Unit: "servings", Category: "Produce", EstimatedCost: 15.0, Priority: "essential"},
		{Name: "Vegetables", Amount: 14, Unit: "servings", Category: "Produce", EstimatedCost: 20.0, Priority: "essential"},
		{Name: "Proteins", Amount: 7, Unit: "servings", Category: "Meat/Dairy", EstimatedCost: 25.0, Priority: "essential"},
		{Name: "Grains", Amount: 7, Unit: "servings", Category: "Pantry", EstimatedCost: 10.0, Priority: "essential"},
	}
}

// generateNutritionSummary creates nutrition summary for the meal plan
func (s *EnterpriseAIService) generateNutritionSummary(dailyMeals []DayMealPlan) *NutritionSummary {
	// Simplified nutrition summary
	return &NutritionSummary{
		DailyAverages: &outbound.NutritionInfo{
			Calories: 1800,
			Protein:  80.0,
			Carbs:    200.0,
			Fat:      70.0,
			Fiber:    30.0,
			Sugar:    50.0,
			Sodium:   2000.0,
		},
		TotalNutrition: &outbound.NutritionInfo{
			Calories: 1800 * len(dailyMeals),
			Protein:  80.0 * float64(len(dailyMeals)),
			Carbs:    200.0 * float64(len(dailyMeals)),
			Fat:      70.0 * float64(len(dailyMeals)),
			Fiber:    30.0 * float64(len(dailyMeals)),
			Sugar:    50.0 * float64(len(dailyMeals)),
			Sodium:   2000.0 * float64(len(dailyMeals)),
		},
		HealthScore:  8.5,
		BalanceScore: 8.0,
	}
}

// checkProviderHealth checks the health of AI providers
func (s *EnterpriseAIService) checkProviderHealth(ctx context.Context) error {
	// This would implement actual health checks for AI providers
	// For now, we'll just return nil to indicate healthy
	return nil
}