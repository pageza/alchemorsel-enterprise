package main

import (
    "fmt"
    "html/template"
    "path/filepath"
)

func main() {
    // Template functions
    funcMap := template.FuncMap{
        "default": func(defaultValue, value interface{}) interface{} {
            if value == nil || value == "" {
                return defaultValue
            }
            return value
        },
    }

    // Template directory path (relative to project root)
    templateDir := "internal/infrastructure/http/server/templates"
    
    // Collect all template files first
    var allFiles []string
    patterns := []string{
        filepath.Join(templateDir, "layout/*.html"),
        filepath.Join(templateDir, "components/*.html"),
        filepath.Join(templateDir, "pages/*.html"),
        filepath.Join(templateDir, "partials/*.html"),
    }
    
    for _, pattern := range patterns {
        matches, err := filepath.Glob(pattern)
        if err != nil {
            fmt.Printf("Failed to glob pattern %s: %v\n", pattern, err)
            return
        }
        allFiles = append(allFiles, matches...)
    }
    
    if len(allFiles) == 0 {
        fmt.Printf("No template files found in %s\n", templateDir)
        return
    }

    fmt.Printf("Found %d template files\n", len(allFiles))

    // Parse all template files at once to handle template dependencies correctly
    _, err := template.New("base").Funcs(funcMap).ParseFiles(allFiles...)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
    } else {
        fmt.Println("Templates parsed successfully")
    }
}