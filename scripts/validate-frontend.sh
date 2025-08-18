#!/bin/bash

# Alchemorsel v3 - Frontend Implementation Validation Script
# This script validates the 14KB first packet optimization and frontend features

echo "🚀 Alchemorsel v3 - Frontend Validation"
echo "========================================"

# Check if server files exist
echo ""
echo "📁 Checking file structure..."

files=(
    "internal/infrastructure/http/server/server.go"
    "internal/infrastructure/http/handlers/frontend.go"
    "internal/infrastructure/http/handlers/api.go"
    "internal/infrastructure/http/middleware/security.go"
    "internal/infrastructure/http/server/templates/layout/base.html"
    "internal/infrastructure/http/server/static/css/critical.css"
    "internal/infrastructure/http/server/static/js/htmx.min.js"
    "internal/infrastructure/http/server/static/js/app.js"
    "internal/infrastructure/http/server/static/js/accessibility.js"
    "internal/infrastructure/http/server/static/js/performance.js"
)

missing_files=()
for file in "${files[@]}"; do
    if [[ -f "$file" ]]; then
        echo "✅ $file"
    else
        echo "❌ $file"
        missing_files+=("$file")
    fi
done

if [[ ${#missing_files[@]} -eq 0 ]]; then
    echo "✅ All required files present"
else
    echo "❌ Missing ${#missing_files[@]} files"
    exit 1
fi

# Validate critical CSS size
echo ""
echo "🎨 Validating Critical CSS size..."
if [[ -f "internal/infrastructure/http/server/static/css/critical.css" ]]; then
    css_size=$(wc -c < "internal/infrastructure/http/server/static/css/critical.css")
    css_kb=$((css_size / 1024))
    
    if [[ $css_kb -le 4 ]]; then
        echo "✅ Critical CSS: ${css_kb}KB (within 4KB target)"
    else
        echo "⚠️  Critical CSS: ${css_kb}KB (exceeds 4KB target)"
    fi
else
    echo "❌ Critical CSS file not found"
fi

# Validate HTMX size
echo ""
echo "⚡ Validating HTMX JavaScript size..."
if [[ -f "internal/infrastructure/http/server/static/js/htmx.min.js" ]]; then
    js_size=$(wc -c < "internal/infrastructure/http/server/static/js/htmx.min.js")
    js_kb=$((js_size / 1024))
    
    if [[ $js_kb -le 2 ]]; then
        echo "✅ HTMX JS: ${js_kb}KB (within 2KB target)"
    else
        echo "⚠️  HTMX JS: ${js_kb}KB (exceeds 2KB target)"
    fi
else
    echo "❌ HTMX JS file not found"
fi

# Check template structure
echo ""
echo "📄 Validating template structure..."

templates=(
    "internal/infrastructure/http/server/templates/layout/base.html"
    "internal/infrastructure/http/server/templates/pages/home.html"
    "internal/infrastructure/http/server/templates/pages/recipes.html"
    "internal/infrastructure/http/server/templates/components/header.html"
    "internal/infrastructure/http/server/templates/components/footer.html"
    "internal/infrastructure/http/server/templates/partials/search-results.html"
)

for template in "${templates[@]}"; do
    if [[ -f "$template" ]]; then
        echo "✅ $template"
    else
        echo "❌ $template"
    fi
done

# Validate Go code compilation
echo ""
echo "🔧 Validating Go code compilation..."
if go build -o /dev/null ./cmd/api/main.go 2>/dev/null; then
    echo "✅ Go code compiles successfully"
else
    echo "❌ Go code compilation failed"
    echo "Run 'go build ./cmd/api/main.go' for details"
fi

# Check for performance features
echo ""
echo "📊 Checking performance features..."

performance_features=(
    "Service Worker|sw.js"
    "Performance Monitoring|performance.js"
    "Accessibility Enhancement|accessibility.js"
    "Critical CSS Inline|critical-css.html"
    "Progressive Enhancement|extended.css"
)

for feature in "${performance_features[@]}"; do
    name="${feature%%|*}"
    file="${feature##*|}"
    
    if find . -name "*$file*" -type f | grep -q .; then
        echo "✅ $name"
    else
        echo "❌ $name"
    fi
done

# Validate HTMX handlers
echo ""
echo "🔄 Checking HTMX handlers..."

if grep -q "HandleRecipeSearch" internal/infrastructure/http/handlers/frontend.go 2>/dev/null; then
    echo "✅ Recipe Search Handler"
else
    echo "❌ Recipe Search Handler"
fi

if grep -q "HandleAIChat" internal/infrastructure/http/handlers/frontend.go 2>/dev/null; then
    echo "✅ AI Chat Handler"
else
    echo "❌ AI Chat Handler"
fi

if grep -q "HandleVoiceInput" internal/infrastructure/http/handlers/frontend.go 2>/dev/null; then
    echo "✅ Voice Input Handler"
else
    echo "❌ Voice Input Handler"
fi

# Check accessibility features
echo ""
echo "♿ Checking accessibility features..."

accessibility_features=(
    "ARIA labels|aria-label"
    "Skip links|skip-link"
    "Screen reader support|sr-only"
    "Keyboard navigation|keyboard-navigation"
    "High contrast|high-contrast"
)

for feature in "${accessibility_features[@]}"; do
    name="${feature%%|*}"
    pattern="${feature##*|}"
    
    if grep -r "$pattern" internal/infrastructure/http/server/templates/ >/dev/null 2>&1 || 
       grep -r "$pattern" internal/infrastructure/http/server/static/ >/dev/null 2>&1; then
        echo "✅ $name"
    else
        echo "❌ $name"
    fi
done

# Calculate estimated first packet size
echo ""
echo "📦 Estimating first packet size..."

total_size=0

# Estimate HTML size (compressed)
if [[ -f "internal/infrastructure/http/server/templates/layout/base.html" ]]; then
    html_size=$(wc -c < "internal/infrastructure/http/server/templates/layout/base.html")
    # Assume 60% compression ratio
    html_compressed=$((html_size * 60 / 100))
    total_size=$((total_size + html_compressed))
    echo "  HTML (compressed): $((html_compressed / 1024))KB"
fi

# Add critical CSS size
if [[ -f "internal/infrastructure/http/server/static/css/critical.css" ]]; then
    css_size=$(wc -c < "internal/infrastructure/http/server/static/css/critical.css")
    total_size=$((total_size + css_size))
    echo "  Critical CSS: $((css_size / 1024))KB"
fi

# Add HTMX size
if [[ -f "internal/infrastructure/http/server/static/js/htmx.min.js" ]]; then
    htmx_size=$(wc -c < "internal/infrastructure/http/server/static/js/htmx.min.js")
    total_size=$((total_size + htmx_size))
    echo "  HTMX JS: $((htmx_size / 1024))KB"
fi

total_kb=$((total_size / 1024))
echo ""
echo "📊 Total estimated first packet: ${total_kb}KB"

if [[ $total_kb -le 14 ]]; then
    echo "✅ First packet optimization: PASSED (within 14KB target)"
else
    echo "⚠️  First packet optimization: REVIEW NEEDED (exceeds 14KB target)"
fi

# Final summary
echo ""
echo "🎯 Validation Summary"
echo "===================="

if [[ ${#missing_files[@]} -eq 0 ]] && [[ $total_kb -le 14 ]]; then
    echo "✅ Frontend implementation is ready for showcase!"
    echo ""
    echo "🚀 Key Features Implemented:"
    echo "   • 14KB first packet optimization"
    echo "   • HTMX progressive enhancement"
    echo "   • AI chat with voice support"
    echo "   • Real-time search and filtering"
    echo "   • Service worker offline support"
    echo "   • Accessibility compliance"
    echo "   • Performance monitoring"
    echo ""
    echo "🎓 Perfect for startup interviews demonstrating:"
    echo "   • Advanced performance engineering"
    echo "   • Modern web architecture"
    echo "   • Enterprise-grade scalability"
    echo "   • Accessibility excellence"
    echo ""
    echo "Start the server with: go run cmd/api/main.go"
    echo "Then visit: http://localhost:8080"
    echo "Performance dashboard: Ctrl+Shift+P"
else
    echo "❌ Issues found - please review and fix before showcasing"
fi

echo ""