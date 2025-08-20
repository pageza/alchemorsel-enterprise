#!/bin/bash

# Release Tools for Alchemorsel v3
# Collection of utilities for release management

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    local missing=()
    
    if ! command_exists git; then
        missing+=("git")
    fi
    
    if ! command_exists jq; then
        missing+=("jq")
    fi
    
    if ! command_exists curl; then
        missing+=("curl")
    fi
    
    if ! command_exists node; then
        missing+=("node")
    fi
    
    if ! command_exists go; then
        missing+=("go")
    fi
    
    if [ ${#missing[@]} -ne 0 ]; then
        log_error "Missing required tools: ${missing[*]}"
        exit 1
    fi
    
    log_success "All prerequisites satisfied"
}

# Get current version
get_current_version() {
    local version=""
    
    # Try to get from git tag
    if git describe --tags --abbrev=0 >/dev/null 2>&1; then
        version=$(git describe --tags --abbrev=0 | sed 's/^v//')
    elif [ -f "VERSION" ]; then
        version=$(cat VERSION)
    elif [ -f "package.json" ]; then
        version=$(jq -r '.version' package.json)
    else
        version="0.0.0"
    fi
    
    echo "$version"
}

# Calculate next version
calculate_next_version() {
    local current_version="$1"
    local release_type="$2"
    
    IFS='.' read -ra ADDR <<< "$current_version"
    local major="${ADDR[0]}"
    local minor="${ADDR[1]}"
    local patch="${ADDR[2]}"
    
    case "$release_type" in
        "major")
            major=$((major + 1))
            minor=0
            patch=0
            ;;
        "minor")
            minor=$((minor + 1))
            patch=0
            ;;
        "patch")
            patch=$((patch + 1))
            ;;
        *)
            log_error "Invalid release type: $release_type"
            exit 1
            ;;
    esac
    
    echo "$major.$minor.$patch"
}

# Analyze commits to determine release type
analyze_commits() {
    log_info "Analyzing commits since last release..."
    
    local last_tag
    last_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
    
    local commit_range
    if [ -z "$last_tag" ]; then
        commit_range="HEAD"
    else
        commit_range="${last_tag}..HEAD"
    fi
    
    local commits
    commits=$(git log --pretty=format:"%s" "$commit_range")
    
    if [ -z "$commits" ]; then
        log_warning "No commits found since last release"
        echo "none"
        return
    fi
    
    log_info "Commits since $last_tag:"
    echo "$commits" | while read -r commit; do
        echo "  - $commit"
    done
    
    # Check for breaking changes
    if echo "$commits" | grep -qE "^(feat|fix|perf|refactor)(\(.+\))?!:"; then
        echo "major"
        return
    fi
    
    # Check for BREAKING CHANGE in commit body
    if git log --pretty=format:"%B" "$commit_range" | grep -q "BREAKING CHANGE:"; then
        echo "major"
        return
    fi
    
    # Check for features
    if echo "$commits" | grep -qE "^feat(\(.+\))?:"; then
        echo "minor"
        return
    fi
    
    # Check for fixes
    if echo "$commits" | grep -qE "^(fix|perf)(\(.+\))?:"; then
        echo "patch"
        return
    fi
    
    # Default to patch for any other changes
    echo "patch"
}

# Generate changelog
generate_changelog() {
    local from_tag="$1"
    local to_tag="$2"
    
    log_info "Generating changelog from $from_tag to $to_tag..."
    
    local commit_range
    if [ "$from_tag" = "HEAD" ]; then
        commit_range="HEAD"
    else
        commit_range="${from_tag}..${to_tag}"
    fi
    
    cat << EOF
## [$to_tag] - $(date +%Y-%m-%d)

EOF
    
    # Get commits and categorize them
    local commits
    commits=$(git log --pretty=format:"%h|%s|%b|%an|%ad" --date=short "$commit_range")
    
    declare -A categories
    categories[feat]="### ✨ Features"
    categories[fix]="### 🐛 Bug Fixes"
    categories[perf]="### ⚡ Performance Improvements"
    categories[security]="### 🔒 Security"
    categories[refactor]="### ♻️ Code Refactoring"
    categories[docs]="### 📚 Documentation"
    categories[test]="### 🧪 Tests"
    categories[build]="### 🏗️ Build System"
    categories[ci]="### 🔄 Continuous Integration"
    categories[chore]="### 🔧 Maintenance"
    categories[style]="### 💄 Styles"
    categories[revert]="### ⏪ Reverts"
    categories[breaking]="### 💥 Breaking Changes"
    categories[other]="### 🔄 Other Changes"
    
    declare -A commit_lists
    
    if [ -n "$commits" ]; then
        while IFS='|' read -r hash subject body author date; do
            local category="other"
            local is_breaking=false
            
            # Check for breaking changes
            if [[ "$subject" =~ ^(feat|fix|perf|refactor)(\(.+\))?!: ]] || [[ "$body" =~ BREAKING[[:space:]]CHANGE: ]]; then
                category="breaking"
                is_breaking=true
            elif [[ "$subject" =~ ^feat(\(.+\))?: ]]; then
                category="feat"
            elif [[ "$subject" =~ ^fix(\(.+\))?: ]]; then
                category="fix"
            elif [[ "$subject" =~ ^perf(\(.+\))?: ]]; then
                category="perf"
            elif [[ "$subject" =~ ^security(\(.+\))?: ]]; then
                category="security"
            elif [[ "$subject" =~ ^refactor(\(.+\))?: ]]; then
                category="refactor"
            elif [[ "$subject" =~ ^docs(\(.+\))?: ]]; then
                category="docs"
            elif [[ "$subject" =~ ^test(\(.+\))?: ]]; then
                category="test"
            elif [[ "$subject" =~ ^build(\(.+\))?: ]]; then
                category="build"
            elif [[ "$subject" =~ ^ci(\(.+\))?: ]]; then
                category="ci"
            elif [[ "$subject" =~ ^chore(\(.+\))?: ]]; then
                category="chore"
            elif [[ "$subject" =~ ^style(\(.+\))?: ]]; then
                category="style"
            elif [[ "$subject" =~ ^revert(\(.+\))?: ]]; then
                category="revert"
            fi
            
            # Clean up subject line
            local clean_subject
            clean_subject=$(echo "$subject" | sed -E 's/^(feat|fix|docs|style|refactor|perf|test|chore|security|build|ci|revert)(\(.+\))?!?:[[:space:]]*//')
            
            # Format the commit entry
            local entry="- $clean_subject ($hash)"
            
            if [ -z "${commit_lists[$category]:-}" ]; then
                commit_lists[$category]=""
            fi
            commit_lists[$category]+="$entry"$'\n'
            
        done <<< "$commits"
        
        # Output categorized commits
        for category in breaking feat fix perf security refactor docs test build ci chore style revert other; do
            if [ -n "${commit_lists[$category]:-}" ]; then
                echo "${categories[$category]}"
                echo ""
                echo "${commit_lists[$category]}"
            fi
        done
    else
        echo "### 🔧 Maintenance"
        echo ""
        echo "- Internal improvements and updates"
        echo ""
    fi
}

# Create release
create_release() {
    local version="$1"
    local release_type="$2"
    
    log_info "Creating release v$version..."
    
    # Get current version for changelog
    local current_version
    current_version=$(get_current_version)
    
    # Generate changelog
    local changelog
    changelog=$(generate_changelog "v$current_version" "v$version")
    
    # Update version files
    log_info "Updating version files..."
    
    # Update VERSION file
    echo "$version" > VERSION
    
    # Update package.json if it exists
    if [ -f "package.json" ]; then
        jq --arg version "$version" '.version = $version' package.json > package.json.tmp
        mv package.json.tmp package.json
    fi
    
    # Create and push tag
    log_info "Creating git tag v$version..."
    git add VERSION package.json 2>/dev/null || true
    git commit -m "chore(release): bump version to v$version" || true
    git tag -a "v$version" -m "Release v$version"
    
    log_success "Release v$version created successfully!"
    log_info "To push the release, run: git push origin main && git push origin v$version"
    
    # Show changelog
    echo ""
    log_info "Generated changelog:"
    echo "$changelog"
}

# Dry run - show what would be released
dry_run() {
    log_info "Performing dry run..."
    
    local current_version
    current_version=$(get_current_version)
    log_info "Current version: $current_version"
    
    local release_type
    release_type=$(analyze_commits)
    log_info "Detected release type: $release_type"
    
    if [ "$release_type" = "none" ]; then
        log_warning "No changes detected, no release needed"
        return
    fi
    
    local next_version
    next_version=$(calculate_next_version "$current_version" "$release_type")
    log_info "Next version would be: $next_version"
    
    # Generate changelog preview
    local changelog
    changelog=$(generate_changelog "v$current_version" "v$next_version")
    
    echo ""
    log_info "Changelog preview:"
    echo "$changelog"
}

# Rollback release
rollback_release() {
    local version="$1"
    
    log_warning "Rolling back release v$version..."
    
    # Delete tag locally
    git tag -d "v$version" 2>/dev/null || true
    
    # Delete tag remotely (requires confirmation)
    read -p "Delete remote tag v$version? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git push --delete origin "v$version" || true
        log_success "Remote tag v$version deleted"
    fi
    
    # Reset to previous commit if this was the last commit
    local last_commit_msg
    last_commit_msg=$(git log -1 --pretty=format:"%s")
    if [[ "$last_commit_msg" =~ chore\(release\):.*bump.*version.*to.*v$version ]]; then
        read -p "Reset to previous commit? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            git reset --hard HEAD~1
            log_success "Reset to previous commit"
        fi
    fi
}

# Main function
main() {
    local command="${1:-help}"
    
    case "$command" in
        "check")
            check_prerequisites
            ;;
        "version")
            echo "$(get_current_version)"
            ;;
        "analyze")
            analyze_commits
            ;;
        "dry-run"|"dryrun")
            check_prerequisites
            dry_run
            ;;
        "release")
            local release_type="${2:-auto}"
            check_prerequisites
            
            local current_version
            current_version=$(get_current_version)
            
            if [ "$release_type" = "auto" ]; then
                release_type=$(analyze_commits)
                if [ "$release_type" = "none" ]; then
                    log_warning "No changes detected, no release needed"
                    exit 0
                fi
            fi
            
            local next_version
            next_version=$(calculate_next_version "$current_version" "$release_type")
            
            create_release "$next_version" "$release_type"
            ;;
        "rollback")
            local version="${2:-}"
            if [ -z "$version" ]; then
                log_error "Please specify version to rollback (e.g., rollback 1.2.3)"
                exit 1
            fi
            rollback_release "$version"
            ;;
        "changelog")
            local from="${2:-$(git describe --tags --abbrev=0 2>/dev/null || echo 'HEAD')}"
            local to="${3:-HEAD}"
            generate_changelog "$from" "$to"
            ;;
        "help"|*)
            cat << EOF
Alchemorsel v3 Release Tools

Usage: $0 <command> [arguments]

Commands:
  check                   Check prerequisites
  version                 Show current version
  analyze                 Analyze commits to determine release type
  dry-run                 Show what would be released without creating it
  release [type]          Create a release (type: major|minor|patch|auto)
  rollback <version>      Rollback a release
  changelog [from] [to]   Generate changelog between versions
  help                    Show this help message

Examples:
  $0 check                    # Check if all tools are available
  $0 dry-run                  # Preview next release
  $0 release auto             # Auto-detect and create release
  $0 release minor            # Force minor release
  $0 rollback 1.2.3           # Rollback version 1.2.3
  $0 changelog v1.0.0 v1.1.0  # Generate changelog between versions

EOF
            ;;
    esac
}

# Run main function with all arguments
main "$@"