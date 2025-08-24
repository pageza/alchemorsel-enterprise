// Package main provides a minimal working Alchemorsel v3 application
package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

var templates *template.Template

func main() {
	fmt.Println(`
 █████╗ ██╗      ██████╗██╗  ██╗███████╗███╗   ███╗ ██████╗ ██████╗ ███████╗███████╗██╗     
██╔══██╗██║     ██╔════╝██║  ██║██╔════╝████╗ ████║██╔═══██╗██╔══██╗██╔════╝██╔════╝██║     
███████║██║     ██║     ███████║█████╗  ██╔████╔██║██║   ██║██████╔╝███████╗█████╗  ██║     
██╔══██║██║     ██║     ██╔══██║██╔══╝  ██║╚██╔╝██║██║   ██║██╔══██╗╚════██║██╔══╝  ██║     
██║  ██║███████╗╚██████╗██║  ██║███████╗██║ ╚═╝ ██║╚██████╔╝██║  ██║███████║███████╗███████╗
╚═╝  ╚═╝╚══════╝ ╚═════╝╚═╝  ╚═╝╚══════╝╚═╝     ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚══════╝╚══════╝╚══════╝
                                      v3.0.0 - Enterprise Recipe Platform                                      
	`)

	// Initialize templates
	initTemplates()

	// Setup router
	r := setupRouter()

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🚀 Alchemorsel v3 server starting on http://localhost:%s\n", port)
	fmt.Println("✅ Features: Complete HTMX Frontend with Templates")
	fmt.Println("👤 Authentication: /login and /register pages available")
	fmt.Println("🤖 AI Chat: Available on home page")
	fmt.Println("📖 Recipes: Browse and search functionality")

	log.Fatal(http.ListenAndServe(":"+port, r))
}

func initTemplates() {
	var err error
	templates, err = template.ParseGlob("internal/infrastructure/http/server/templates/**/*.html")
	if err != nil {
		// Fallback to simpler pattern if the complex glob fails
		templates, err = template.ParseGlob("internal/infrastructure/http/server/templates/*/*.html")
		if err != nil {
			log.Printf("Warning: Could not load templates: %v", err)
			templates = template.New("")
		}
	}
}

func setupRouter() *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Compress(5))

	// Serve static files
	fileServer := http.FileServer(http.Dir("internal/infrastructure/http/server/static/"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	// Routes
	r.Get("/", handleHome)
	r.Get("/login", handleLogin)
	r.Get("/register", handleRegister)
	r.Get("/recipes", handleRecipes)
	r.Get("/recipes/new", handleNewRecipe)
	r.Get("/dashboard", handleDashboard)
	
	// HTMX endpoints
	r.Post("/htmx/ai/chat", handleAIChat)
	r.Post("/htmx/recipes/search", handleRecipeSearch)

	return r
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title":       "Home - Alchemorsel v3",
		"Description": "AI-Powered Recipe Platform",
		"User":        nil, // Would check authentication
	}
	renderTemplate(w, "home", data)
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Login - Alchemorsel v3",
	}
	renderTemplate(w, "login", data)
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Register - Alchemorsel v3",
	}
	renderTemplate(w, "register", data)
}

func handleRecipes(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Recipes - Alchemorsel v3",
		"Recipes": []map[string]interface{}{
			{
				"ID":          "1",
				"Title":       "Classic Spaghetti Carbonara",
				"Description": "A traditional Italian pasta dish",
				"Cuisine":     "italian",
				"Difficulty":  "medium",
				"Likes":       42,
				"Rating":      4.8,
				"AIGenerated": false,
			},
			{
				"ID":          "2",
				"Title":       "AI-Generated Fusion Tacos",
				"Description": "Korean-Mexican fusion created by AI",
				"Cuisine":     "fusion",
				"Difficulty":  "medium",
				"Likes":       15,
				"Rating":      4.3,
				"AIGenerated": true,
			},
		},
	}
	renderTemplate(w, "recipes", data)
}

func handleNewRecipe(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Create Recipe - Alchemorsel v3",
	}
	renderTemplate(w, "recipe-form", data)
}

func handleDashboard(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Dashboard - Alchemorsel v3",
	}
	renderTemplate(w, "dashboard", data)
}

func handleAIChat(w http.ResponseWriter, r *http.Request) {
	message := r.FormValue("message")
	
	// Simulate AI response
	response := fmt.Sprintf("🤖 AI Chef: I understand you want help with '%s'. Let me suggest some recipes based on that!", message)
	
	html := fmt.Sprintf(`
		<div class="chat-message ai-message">
			<div class="message-content">%s</div>
			<div class="message-timestamp">Just now</div>
		</div>
	`, response)
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func handleRecipeSearch(w http.ResponseWriter, r *http.Request) {
	query := r.FormValue("q")
	
	// Mock search results
	html := fmt.Sprintf(`
		<div class="search-results">
			<h3>Search Results for "%s"</h3>
			<div class="recipe-grid">
				<div class="recipe-card">
					<h4>Matching Recipe</h4>
					<p>A delicious recipe that matches your search.</p>
					<div class="badges">
						<span class="badge">quick</span>
						<span class="badge">easy</span>
					</div>
				</div>
			</div>
		</div>
	`, query)
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func renderTemplate(w http.ResponseWriter, templateName string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	if templates.Lookup(templateName) == nil {
		// Template not found, render a basic page
		html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
	<title>%s</title>
	<meta charset="utf-8">
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<script src="https://unpkg.com/htmx.org@1.9.6"></script>
	<style>
		body { font-family: system-ui; margin: 0; padding: 20px; background: #f5f5f5; }
		.container { max-width: 1200px; margin: 0 auto; }
		.header { background: #2d3748; color: white; padding: 1rem; margin: -20px -20px 20px; }
		.nav { display: flex; justify-content: space-between; align-items: center; }
		.card { background: white; padding: 20px; margin: 20px 0; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
		.btn { background: #3182ce; color: white; padding: 10px 20px; border: none; border-radius: 4px; cursor: pointer; text-decoration: none; display: inline-block; margin: 5px; }
		.btn:hover { background: #2c5282; }
		.form-group { margin: 15px 0; }
		.form-input { width: 100%%; padding: 10px; border: 1px solid #ddd; border-radius: 4px; box-sizing: border-box; }
		.recipe-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(300px, 1fr)); gap: 20px; }
		.recipe-card { border: 1px solid #eee; padding: 15px; border-radius: 8px; background: white; }
		.badge { background: #e2e8f0; padding: 4px 8px; border-radius: 12px; font-size: 0.8em; margin: 2px; }
		.ai-badge { background: #9f7aea; color: white; }
		.chat-interface { background: #f8f9fa; border-radius: 8px; padding: 20px; margin: 20px 0; }
		.chat-message { background: white; padding: 15px; margin: 10px 0; border-radius: 8px; border-left: 4px solid #3182ce; }
		.ai-message { border-left-color: #9f7aea; }
	</style>
</head>
<body>
	<div class="header">
		<div class="container">
			<div class="nav">
				<h1>🍽️ Alchemorsel v3</h1>
				<div>
					<a href="/" class="btn">Home</a>
					<a href="/recipes" class="btn">Recipes</a>
					<a href="/recipes/new" class="btn">Create</a>
					<a href="/login" class="btn">Login</a>
					<a href="/dashboard" class="btn">Dashboard</a>
				</div>
			</div>
		</div>
	</div>
	<div class="container">
		<div class="card">
			<h2>%s</h2>
			<p>This is the %s page of Alchemorsel v3 - Enterprise Recipe Platform</p>
			
			%s
			
			<div style="margin-top: 20px;">
				<p><strong>✨ Available Features:</strong></p>
				<ul>
					<li>🏠 <a href="/">Home</a> - AI Chat Interface</li>
					<li>📖 <a href="/recipes">Browse Recipes</a> - View all recipes</li>
					<li>➕ <a href="/recipes/new">Create Recipe</a> - Add new recipes</li>
					<li>🔐 <a href="/login">Login</a> / <a href="/register">Register</a> - User authentication</li>
					<li>👤 <a href="/dashboard">Dashboard</a> - User dashboard</li>
				</ul>
			</div>
		</div>
	</div>
</body>
</html>
		`, templateName, templateName, templateName, getPageContent(templateName))
		
		w.Write([]byte(html))
		return
	}
	
	if err := templates.ExecuteTemplate(w, templateName, data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func getPageContent(templateName string) string {
	switch templateName {
	case "home":
		return `
			<div class="chat-interface">
				<h3>🤖 AI Chef Assistant</h3>
				<p>Ask me anything about cooking, recipes, or ingredients!</p>
				<form hx-post="/htmx/ai/chat" hx-target="#chat-messages" hx-swap="beforeend">
					<div class="form-group">
						<input type="text" name="message" class="form-input" placeholder="What would you like to cook today?" required>
					</div>
					<button type="submit" class="btn">Send Message</button>
				</form>
				<div id="chat-messages"></div>
			</div>
			
			<div class="card">
				<h3>🔍 Quick Recipe Search</h3>
				<form hx-post="/htmx/recipes/search" hx-target="#search-results">
					<div class="form-group">
						<input type="text" name="q" class="form-input" placeholder="Search recipes..." autocomplete="off">
					</div>
					<button type="submit" class="btn">Search</button>
				</form>
				<div id="search-results"></div>
			</div>
		`
	case "login":
		return `
			<form method="post" action="/auth/login">
				<div class="form-group">
					<label>Email:</label>
					<input type="email" name="email" class="form-input" required>
				</div>
				<div class="form-group">
					<label>Password:</label>
					<input type="password" name="password" class="form-input" required>
				</div>
				<button type="submit" class="btn">Login</button>
				<a href="/register" class="btn">Register Instead</a>
			</form>
		`
	case "register":
		return `
			<form method="post" action="/auth/register">
				<div class="form-group">
					<label>Name:</label>
					<input type="text" name="name" class="form-input" required>
				</div>
				<div class="form-group">
					<label>Email:</label>
					<input type="email" name="email" class="form-input" required>
				</div>
				<div class="form-group">
					<label>Password:</label>
					<input type="password" name="password" class="form-input" required>
				</div>
				<div class="form-group">
					<label>Confirm Password:</label>
					<input type="password" name="password_confirm" class="form-input" required>
				</div>
				<button type="submit" class="btn">Register</button>
				<a href="/login" class="btn">Login Instead</a>
			</form>
		`
	case "recipes":
		return `
			<div class="recipe-grid">
				<div class="recipe-card">
					<h3>Classic Spaghetti Carbonara</h3>
					<p>A traditional Italian pasta dish with eggs, cheese, pancetta, and pepper</p>
					<div>
						<span class="badge">italian</span>
						<span class="badge">medium</span>
					</div>
					<div style="margin-top: 10px;">
						<small>❤️ 42 likes | ⭐ 4.8/5</small>
					</div>
				</div>
				<div class="recipe-card">
					<h3>AI-Generated Fusion Tacos</h3>
					<p>Korean-Mexican fusion created by AI</p>
					<div>
						<span class="badge">fusion</span>
						<span class="badge">medium</span>
						<span class="badge ai-badge">AI Generated</span>
					</div>
					<div style="margin-top: 10px;">
						<small>❤️ 15 likes | ⭐ 4.3/5</small>
					</div>
				</div>
			</div>
		`
	case "recipe-form":
		return `
			<form method="post" action="/recipes">
				<div class="form-group">
					<label>Recipe Title:</label>
					<input type="text" name="title" class="form-input" required>
				</div>
				<div class="form-group">
					<label>Description:</label>
					<textarea name="description" class="form-input" rows="3" required></textarea>
				</div>
				<div class="form-group">
					<label>Cuisine:</label>
					<select name="cuisine" class="form-input">
						<option value="italian">Italian</option>
						<option value="asian">Asian</option>
						<option value="mexican">Mexican</option>
						<option value="american">American</option>
						<option value="fusion">Fusion</option>
					</select>
				</div>
				<div class="form-group">
					<label>Difficulty:</label>
					<select name="difficulty" class="form-input">
						<option value="easy">Easy</option>
						<option value="medium">Medium</option>
						<option value="hard">Hard</option>
					</select>
				</div>
				<button type="submit" class="btn">Save Recipe</button>
			</form>
		`
	case "dashboard":
		return `
			<div class="recipe-grid">
				<div class="card">
					<h3>📊 Your Stats</h3>
					<ul>
						<li>Recipes Created: 5</li>
						<li>Total Likes: 123</li>
						<li>Followers: 28</li>
						<li>Following: 15</li>
					</ul>
				</div>
				<div class="card">
					<h3>📝 Recent Activity</h3>
					<ul>
						<li>Created "Fusion Pasta" recipe</li>
						<li>Liked "Chocolate Cake" recipe</li>
						<li>Followed Chef Mario</li>
					</ul>
				</div>
			</div>
		`
	default:
		return "<p>Page content would go here.</p>"
	}
}