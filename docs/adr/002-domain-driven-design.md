# ADR-002: Domain-Driven Design Implementation

## Status
Accepted

## Context
Alchemorsel v3 is a complex domain with multiple business concepts and rules:
- Recipe management with ingredients, instructions, nutrition
- User management with preferences, social features
- AI-powered recipe generation and analysis
- Social interactions (likes, comments, follows)
- Content moderation and quality control

Traditional CRUD-based approaches fall short for:
- Complex business rules and validation
- Rich domain behavior and invariants
- Domain expert knowledge capture
- Cross-aggregate consistency
- Domain events and integration

## Decision
We will implement Domain-Driven Design (DDD) principles throughout Alchemorsel v3.

### Core DDD Concepts:

#### 1. Bounded Contexts
- **Recipe Management**: Core recipe CRUD, ingredients, instructions
- **User Management**: Authentication, profiles, preferences
- **AI Services**: Recipe generation, nutrition analysis, recommendations
- **Social Platform**: Likes, comments, follows, feeds
- **Content Moderation**: Quality control, spam detection

#### 2. Domain Model Structure
```
internal/domain/
├── recipe/
│   ├── entity.go          # Recipe aggregate root
│   ├── value_objects.go   # Ingredient, Instruction, Nutrition
│   ├── events.go          # Domain events
│   ├── repository.go      # Repository interface
│   └── service.go         # Domain services
├── user/
│   ├── entity.go          # User aggregate root
│   ├── value_objects.go   # Profile, Preferences
│   └── events.go          # User events
├── ai/
│   ├── generation.go      # AI generation domain logic
│   └── analysis.go        # Nutrition/quality analysis
└── shared/
    ├── events.go          # Shared event interfaces
    └── value_objects.go   # Common value objects
```

#### 3. Aggregates and Entities

**Recipe Aggregate:**
- Root: Recipe entity
- Contains: Ingredients, Instructions, Images, Nutrition
- Invariants: Must have title, at least one ingredient when published
- Events: RecipeCreated, RecipePublished, RecipeLiked

**User Aggregate:**
- Root: User entity
- Contains: Profile, Preferences, Social stats
- Invariants: Unique email/username, valid email format
- Events: UserRegistered, ProfileUpdated, UserFollowed

#### 4. Value Objects
- **Ingredient**: Name, Amount, Unit (immutable)
- **Instruction**: Step number, Description, Duration
- **EmailAddress**: Validated email with format checking
- **Rating**: Value (1-5), Comment, Timestamp

#### 5. Domain Services
- **RecipeRecommendationService**: Complex recommendation logic
- **NutritionCalculationService**: Nutrition analysis across ingredients
- **RecipeValidationService**: Publishing quality checks

#### 6. Domain Events
- Enable loose coupling between bounded contexts
- Support eventual consistency
- Facilitate integration patterns
- Enable audit trails and analytics

## Consequences

### Positive:
- **Rich Domain Model**: Business logic lives where it belongs
- **Ubiquitous Language**: Clear communication between developers and domain experts
- **Maintainability**: Changes to business rules are localized
- **Testability**: Domain logic is pure and easily testable
- **Scalability**: Clear boundaries enable independent scaling

### Negative:
- **Complexity**: More sophisticated modeling approach
- **Learning Curve**: Team needs DDD knowledge
- **Over-Engineering Risk**: Simple operations may become complex

### Implementation Guidelines:

#### 1. Aggregate Design Rules
- Aggregate roots are the only entry points
- References between aggregates use IDs only
- Keep aggregates small and focused
- Ensure consistency boundaries align with business rules

#### 2. Repository Pattern
```go
type RecipeRepository interface {
    Save(ctx context.Context, recipe *Recipe) error
    FindByID(ctx context.Context, id RecipeID) (*Recipe, error)
    FindByAuthor(ctx context.Context, authorID UserID) ([]*Recipe, error)
}
```

#### 3. Domain Events
```go
type DomainEvent interface {
    EventName() string
    OccurredAt() time.Time
    AggregateID() string
}

type RecipePublishedEvent struct {
    RecipeID    RecipeID
    AuthorID    UserID
    PublishedAt time.Time
}
```

#### 4. Specifications Pattern
```go
type RecipeSpecification interface {
    IsSatisfiedBy(recipe *Recipe) bool
}

type PublishableRecipeSpec struct{}

func (s PublishableRecipeSpec) IsSatisfiedBy(recipe *Recipe) bool {
    return len(recipe.Ingredients()) > 0 && 
           len(recipe.Instructions()) > 0 &&
           recipe.Title() != ""
}
```

## Integration with Hexagonal Architecture

DDD complements our hexagonal architecture:
- **Domain Layer**: Core DDD concepts (entities, value objects, domain services)
- **Application Layer**: Use cases orchestrating domain operations
- **Infrastructure Layer**: Repository implementations, event publishers

## Anti-Patterns to Avoid

1. **Anemic Domain Model**: Avoid entities with only getters/setters
2. **Large Aggregates**: Keep aggregates focused and small
3. **Domain Logic in Application Services**: Business rules belong in the domain
4. **Ignoring Bounded Contexts**: Don't share models across contexts

## Related Decisions
- ADR-001: Hexagonal Architecture
- ADR-003: Event-Driven Architecture
- ADR-004: Repository Pattern Implementation

## References
- [Domain-Driven Design by Eric Evans](https://domainlanguage.com/ddd/)
- [Implementing Domain-Driven Design by Vaughn Vernon](https://vaughnvernon.co/?page_id=168)
- [DDD Community](https://dddcommunity.org/)