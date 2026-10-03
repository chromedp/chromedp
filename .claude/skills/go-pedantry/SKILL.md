---
name: go-pedantry
description: "This skill should be used when the user is writing Go code and needs guidance on Go-specific pedantry: error wrapping with fmt.Errorf and %w, interface design (accept interfaces return structs), package naming conventions, struct field ordering, receiver naming, golangci-lint configuration, and Go-specific patterns that go beyond universal principles."
version: 1.0.0
---

# Go Is Opinionated -- So Are We

Go is a language that already has strong opinions: `gofmt`, short variable names, explicit error handling, no exceptions. But the language's opinions stop at syntax. How you wrap errors, design interfaces, name packages, order struct fields, and configure your linter -- these are decisions Go leaves to you. This skill makes those decisions for you, because inconsistency in these areas is what turns a Go codebase from clean to chaotic.

## Error Wrapping: Always `%w`, Never `%s` or `%v`

When wrapping an error with `fmt.Errorf`, ALWAYS use `%w`. This preserves the error chain so that `errors.Is()` and `errors.As()` work. Using `%s` or `%v` converts the error to a string and destroys the chain. This is not a style preference -- it is the difference between errors that can be programmatically handled and errors that can only be logged.

```go
// BAD -- error chain is destroyed, errors.Is() will not work
func GetUser(ctx context.Context, id string) (*User, error) {
    user, err := db.QueryUser(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get user %s: %s", id, err.Error())
    }
    return user, nil
}

// BAD -- %v also destroys the chain
func GetUser(ctx context.Context, id string) (*User, error) {
    user, err := db.QueryUser(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get user %s: %v", id, err)
    }
    return user, nil
}
```

```go
// GOOD -- %w preserves the error chain
func GetUser(ctx context.Context, id string) (*User, error) {
    user, err := db.QueryUser(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("getting user %s: %w", id, err)
    }
    return user, nil
}

// Now callers can inspect the error chain:
user, err := GetUser(ctx, "usr_123")
if errors.Is(err, sql.ErrNoRows) {
    // handle not found
}
```

**Error message conventions:**
- Start with a lowercase verb in gerund form: "getting user", "parsing config", "connecting to database"
- Do NOT start with "failed to" or "error" -- the fact that it is an error is already clear from the context
- Add identifying information: IDs, names, paths -- whatever helps debug the issue
- Keep it terse: `"getting user %s: %w"` not `"an error occurred while attempting to retrieve the user with ID %s from the database: %w"`

## Error Variable Naming

Sentinel errors (package-level error values) follow a strict naming convention:

```go
// GOOD -- exported, Err prefix, PascalCase after prefix
var (
    ErrNotFound      = errors.New("not found")
    ErrAlreadyExists = errors.New("already exists")
    ErrInvalidInput  = errors.New("invalid input")
    ErrUnauthorized  = errors.New("unauthorized")
    ErrRateLimited   = errors.New("rate limited")
)
```

```go
// BAD -- every one of these violates the convention
var (
    NotFoundError = errors.New("not found")     // wrong: no Err prefix
    errnotfound   = errors.New("not found")     // wrong: unexported + no camelCase
    NOT_FOUND     = errors.New("not found")     // wrong: not Go convention
    UserNotFound  = errors.New("user not found") // wrong: no Err prefix
)
```

Custom error types follow the same prefix pattern:

```go
// GOOD
type ErrValidation struct {
    Field   string
    Message string
}

func (e *ErrValidation) Error() string {
    return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}
```

## Interface Design

### Accept Interfaces, Return Structs

Functions accept interfaces so they can work with any implementation. Functions return concrete structs so the caller has the full type. This is the fundamental Go interface rule.

```go
// BAD -- returning an interface hides the concrete type
func NewUserService(db Database) UserServiceInterface {
    return &UserService{db: db}
}

// BAD -- accepting a concrete type prevents testing and alternative implementations
func ProcessOrder(service *StripeService, order *Order) error {
    return service.Charge(order.Total)
}
```

```go
// GOOD -- accept interface, return struct
func NewUserService(db UserRepository) *UserService {
    return &UserService{db: db}
}

// GOOD -- accept interface for the dependency
func ProcessOrder(payment PaymentProcessor, order *Order) error {
    return payment.Charge(order.Total)
}
```

### Keep Interfaces Small

Go interfaces should be small. One to three methods. If your interface has more than three methods, it is probably doing too much.

```go
// BAD -- too many methods, too specific
type UserManager interface {
    GetUser(ctx context.Context, id string) (*User, error)
    CreateUser(ctx context.Context, input CreateUserInput) (*User, error)
    UpdateUser(ctx context.Context, id string, input UpdateUserInput) (*User, error)
    DeleteUser(ctx context.Context, id string) error
    ListUsers(ctx context.Context, opts ListOptions) ([]*User, error)
    SearchUsers(ctx context.Context, query string) ([]*User, error)
    CountUsers(ctx context.Context) (int, error)
}
```

```go
// GOOD -- small, focused interfaces
type UserReader interface {
    GetUser(ctx context.Context, id string) (*User, error)
}

type UserWriter interface {
    CreateUser(ctx context.Context, input CreateUserInput) (*User, error)
    UpdateUser(ctx context.Context, id string, input UpdateUserInput) (*User, error)
    DeleteUser(ctx context.Context, id string) error
}

type UserLister interface {
    ListUsers(ctx context.Context, opts ListOptions) ([]*User, error)
}
```

### Define Interfaces at the Consumer Side

Interfaces belong where they are used, not where they are implemented. The consumer defines what it needs; the implementation satisfies it without knowing.

```go
// BAD -- interface defined in the implementation package
// package database
type UserStore interface {
    Get(ctx context.Context, id string) (*User, error)
    Save(ctx context.Context, user *User) error
}

type PostgresUserStore struct { ... }
```

```go
// GOOD -- interface defined where it is used
// package handler (the consumer)
type UserGetter interface {
    Get(ctx context.Context, id string) (*User, error)
}

type GetUserHandler struct {
    users UserGetter  // accepts interface
}

// package database (the implementation)
type PostgresUserStore struct { ... }

func (s *PostgresUserStore) Get(ctx context.Context, id string) (*User, error) { ... }
// PostgresUserStore satisfies handler.UserGetter without importing it
```

### Single-Method Interface Naming

Single-method interfaces are named: Verb + "er".

| Method | Interface Name |
|--------|---------------|
| `Read()` | `Reader` |
| `Write()` | `Writer` |
| `Close()` | `Closer` |
| `Format()` | `Formatter` |
| `Validate()` | `Validator` |
| `Handle()` | `Handler` |
| `Encode()` | `Encoder` |

## Package Naming

Go packages have strict naming conventions that the community enforces by reputation.

**Rules:**
1. Short, lowercase, single word: `user`, `auth`, `config`, `order`
2. No underscores, no camelCase: `userservice` not `userService` or `user_service`
3. No stuttering: the package name is part of the qualified name. `user.Service` not `user.UserService`
4. No generic names: `utils`, `helpers`, `common`, `misc`, `shared`, `base` -- these are organizational failures

```go
// BAD -- package naming violations
package userService        // camelCase
package user_service       // underscores
package utils              // meaningless
package common             // meaningless
package helpers            // meaningless

// In user package:
type UserService struct{}  // stutters: user.UserService
type UserRepository struct{} // stutters: user.UserRepository
```

```go
// GOOD -- clean package names
package user

type Service struct{}     // user.Service -- reads naturally
type Repository struct{}  // user.Repository -- no stutter

package auth
type Token struct{}       // auth.Token
type Middleware struct{}   // auth.Middleware

package order
type Processor struct{}   // order.Processor
```

## Struct Field Ordering

Struct fields are ordered by purpose, not alphabetically. Exported fields come first. Related fields are grouped. Groups are separated by blank lines.

```go
// BAD -- no logical grouping
type Server struct {
    addr          string
    logger        *slog.Logger
    TLS           bool
    db            *sql.DB
    Port          int
    Host          string
    readTimeout   time.Duration
    cache         *Cache
    WriteTimeout  time.Duration
    maxConns      int
}
```

```go
// GOOD -- grouped by purpose, exported first within groups, tags consistent
type Server struct {
    // Configuration (exported)
    Host         string        `json:"host" yaml:"host"`
    Port         int           `json:"port" yaml:"port"`
    TLS          bool          `json:"tls"  yaml:"tls"`
    ReadTimeout  time.Duration `json:"read_timeout" yaml:"read_timeout"`
    WriteTimeout time.Duration `json:"write_timeout" yaml:"write_timeout"`
    MaxConns     int           `json:"max_conns" yaml:"max_conns"`

    // Dependencies (unexported)
    db     *sql.DB
    cache  *Cache
    logger *slog.Logger
}
```

**Rules:**
1. Group by purpose (configuration, dependencies, state, internal)
2. Exported fields first within each group
3. Blank lines between field groups
4. Struct tags are consistent in style (`json:"snake_case"`)
5. Align struct tags only if the struct is small (5 fields or fewer) and will not change often

## Receiver Naming

Receivers are one or two lowercase letters derived from the type name. They are consistent across ALL methods on the type. Never use `this` or `self`.

```go
// BAD -- inconsistent receivers, verbose names
func (server *Server) Start() error { ... }
func (s *Server) Stop() error { ... }
func (this *Server) Addr() string { ... }
func (self *Server) IsRunning() bool { ... }
```

```go
// GOOD -- consistent single-letter receiver
func (s *Server) Start() error { ... }
func (s *Server) Stop() error { ... }
func (s *Server) Addr() string { ... }
func (s *Server) IsRunning() bool { ... }
```

| Type | Receiver |
|------|----------|
| `Server` | `s` |
| `User` | `u` |
| `OrderProcessor` | `op` |
| `ConfigService` | `cs` |
| `Handler` | `h` |
| `Client` | `c` |
| `Repository` | `r` |

Two-letter receivers are for when the single letter would be ambiguous (multiple types starting with the same letter in the same file).

## Context: Always First Parameter

`context.Context` is always the first parameter. Always named `ctx`. Never stored in a struct.

```go
// BAD -- context in wrong position
func GetUser(id string, ctx context.Context) (*User, error) { ... }

// BAD -- context stored in struct
type Service struct {
    ctx context.Context
    db  *sql.DB
}

// BAD -- context not named ctx
func GetUser(c context.Context, id string) (*User, error) { ... }
```

```go
// GOOD -- context first, named ctx
func GetUser(ctx context.Context, id string) (*User, error) { ... }

func (s *Service) ProcessOrder(ctx context.Context, order *Order) error { ... }
```

## golangci-lint Configuration

```yaml
# .golangci.yml
linters:
  enable:
    - errcheck       # unchecked errors
    - gosimple       # simplifications
    - govet          # suspicious constructs
    - ineffassign    # unused assignments
    - staticcheck    # advanced static analysis
    - unused         # unused code
    - gocritic       # opinionated checks
    - revive         # flexible linter
    - errorlint      # error wrapping correctness
    - nilerr         # returning nil when err is not nil
    - wrapcheck      # errors from external packages are wrapped
    - prealloc       # slice pre-allocation
    - unconvert      # unnecessary type conversions
    - tenv           # os.Setenv in tests (use t.Setenv)

linters-settings:
  gocritic:
    enabled-checks:
      - appendAssign
      - argOrder
      - badCall
      - badCond
      - dupArg
      - dupBranchBody
      - dupCase
      - elseif
      - flagDeref
      - nilValReturn
      - singleCaseSwitch
      - underef
      - unnecessaryBlock

  revive:
    rules:
      - name: blank-imports
      - name: context-as-argument
      - name: context-keys-type
      - name: dot-imports
      - name: error-naming
      - name: error-return
      - name: error-strings
      - name: exported
      - name: increment-decrement
      - name: indent-error-flow
      - name: package-comments
      - name: range
      - name: receiver-naming
      - name: time-naming
      - name: var-naming
      - name: unexported-return

  errorlint:
    errorf: true          # check fmt.Errorf uses %w
    asserts: true         # check errors.As uses pointer
    comparison: true      # check errors.Is instead of ==
```

## Examples

Working implementations in `examples/`:
- **`examples/error-wrapping.md`** -- Complete Go examples showing proper error wrapping with `%w`, error message conventions, sentinel errors, and custom error types with `errors.Is`/`errors.As` patterns
- **`examples/interface-design.md`** -- Go examples showing interface design patterns: accept interfaces/return structs, small interfaces, consumer-side definition, and the Verb+er naming convention

## Review Checklist

When reviewing Go code:

- [ ] All error wrapping uses `fmt.Errorf("context: %w", err)` -- never `%s` or `%v` for errors
- [ ] Error messages start with lowercase, no "failed to" or "error" prefix, include identifying info (IDs, paths)
- [ ] Sentinel errors use `Err` prefix with PascalCase: `ErrNotFound`, `ErrInvalidInput`
- [ ] Functions accept interfaces and return concrete structs
- [ ] Interfaces are small (1-3 methods) and defined at the consumer side
- [ ] Single-method interfaces follow Verb+er naming (`Reader`, `Writer`, `Validator`)
- [ ] Package names are short, lowercase, single word -- no `utils`, `helpers`, `common`
- [ ] No name stuttering: `user.Service` not `user.UserService`
- [ ] Struct fields are grouped by purpose with exported fields first and blank lines between groups
- [ ] Struct tags are consistent in style (`json:"snake_case"`)
- [ ] Receiver names are 1-2 lowercase letters derived from the type name, consistent across all methods
- [ ] No `this` or `self` receivers
- [ ] `context.Context` is always the first parameter, always named `ctx`, never stored in a struct
- [ ] `golangci-lint` is configured with at minimum: errcheck, gosimple, govet, staticcheck, unused, gocritic, revive, errorlint