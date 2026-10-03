# Interface Design Patterns in Go

Go examples showing the core interface design principles: accept interfaces/return structs, small interfaces, consumer-side definition, and the Verb+er naming convention. Every example demonstrates why Go interfaces work differently from Java/C# interfaces and how to use them idiomatically.

## Accept Interfaces, Return Structs

### Bad -- Returning Interfaces

```go
package user

// UserStore is defined in the same package as the implementation -- wrong location
type UserStore interface {
	Get(ctx context.Context, id string) (*User, error)
	Save(ctx context.Context, user *User) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, opts ListOptions) ([]*User, error)
}

// Returns an interface -- hides the concrete type, prevents access to implementation-specific methods
func NewPostgresStore(db *sql.DB) UserStore {
	return &postgresStore{db: db}
}

type postgresStore struct {
	db *sql.DB
}

func (s *postgresStore) Get(ctx context.Context, id string) (*User, error) { ... }
func (s *postgresStore) Save(ctx context.Context, user *User) error { ... }
func (s *postgresStore) Delete(ctx context.Context, id string) error { ... }
func (s *postgresStore) List(ctx context.Context, opts ListOptions) ([]*User, error) { ... }
```

### Good -- Returning Concrete Struct

```go
package postgres

import (
	"context"
	"database/sql"
)

// UserStore is a concrete struct -- no interface in this package
type UserStore struct {
	db *sql.DB
}

// Returns *UserStore, not an interface
func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

func (s *UserStore) Get(ctx context.Context, id string) (*User, error) {
	row := s.db.QueryRowContext(ctx, "SELECT id, name, email FROM users WHERE id = $1", id)

	var user User
	if err := row.Scan(&user.ID, &user.Name, &user.Email); err != nil {
		return nil, fmt.Errorf("getting user %s: %w", id, err)
	}

	return &user, nil
}

func (s *UserStore) Save(ctx context.Context, user *User) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO users (id, name, email) VALUES ($1, $2, $3) ON CONFLICT (id) DO UPDATE SET name = $2, email = $3",
		user.ID, user.Name, user.Email,
	)
	if err != nil {
		return fmt.Errorf("saving user %s: %w", user.ID, err)
	}
	return nil
}

func (s *UserStore) Delete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM users WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("deleting user %s: %w", id, err)
	}
	return nil
}

func (s *UserStore) List(ctx context.Context, opts ListOptions) ([]*User, error) {
	// ...
	return users, nil
}
```

## Consumer-Side Interface Definition

The consumer defines the interface it needs. The implementation satisfies it implicitly.

```go
// package handler -- the CONSUMER defines the interface

package handler

import "context"

// UserGetter is what the handler needs -- just one method
type UserGetter interface {
	Get(ctx context.Context, id string) (*User, error)
}

type GetUserHandler struct {
	users UserGetter  // accepts interface
}

func NewGetUserHandler(users UserGetter) *GetUserHandler {
	return &GetUserHandler{users: users}
}

func (h *GetUserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	user, err := h.users.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, user)
}
```

```go
// package service -- another CONSUMER, different interface

package service

import "context"

// UserReadWriter is what the service needs -- two methods
type UserReadWriter interface {
	Get(ctx context.Context, id string) (*User, error)
	Save(ctx context.Context, user *User) error
}

type UserService struct {
	users UserReadWriter  // accepts interface
}

func NewUserService(users UserReadWriter) *UserService {
	return &UserService{users: users}
}

func (s *UserService) UpdateEmail(ctx context.Context, id string, newEmail string) error {
	user, err := s.users.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("updating email for user %s: %w", id, err)
	}

	user.Email = newEmail

	if err := s.users.Save(ctx, user); err != nil {
		return fmt.Errorf("saving user %s after email update: %w", id, err)
	}

	return nil
}
```

```go
// Wiring -- the concrete type satisfies both interfaces without knowing about them

package main

func main() {
	db := connectDB()
	store := postgres.NewUserStore(db)  // returns *postgres.UserStore

	// *postgres.UserStore satisfies handler.UserGetter (has Get method)
	getUserHandler := handler.NewGetUserHandler(store)

	// *postgres.UserStore satisfies service.UserReadWriter (has Get and Save methods)
	userService := service.NewUserService(store)

	// ...
}
```

**Why this works:** `postgres.UserStore` has `Get()` and `Save()` methods. The handler only needs `Get()`, so it defines a one-method interface. The service needs `Get()` and `Save()`, so it defines a two-method interface. Neither interface is declared in the `postgres` package. Neither consumer imports the other. The concrete type satisfies both implicitly.

## Small Interfaces and Composition

### Bad -- One Giant Interface

```go
type Storage interface {
	GetUser(ctx context.Context, id string) (*User, error)
	SaveUser(ctx context.Context, user *User) error
	DeleteUser(ctx context.Context, id string) error
	GetOrder(ctx context.Context, id string) (*Order, error)
	SaveOrder(ctx context.Context, order *Order) error
	GetProduct(ctx context.Context, id string) (*Product, error)
	SaveProduct(ctx context.Context, product *Product) error
	RunMigrations(ctx context.Context) error
	Ping(ctx context.Context) error
	Close() error
}
```

### Good -- Composed Small Interfaces

```go
// Small, focused interfaces
type Reader interface {
	Read(ctx context.Context, id string) (*Entity, error)
}

type Writer interface {
	Write(ctx context.Context, entity *Entity) error
}

type Deleter interface {
	Delete(ctx context.Context, id string) error
}

// Compose when a consumer needs multiple capabilities
type ReadWriter interface {
	Reader
	Writer
}

type ReadWriteDeleter interface {
	Reader
	Writer
	Deleter
}

// Health check -- separate concern
type HealthChecker interface {
	Ping(ctx context.Context) error
}
```

## Testing with Interfaces

Consumer-side interfaces make testing trivial. The test defines a mock that satisfies just the interface it needs.

```go
package handler_test

import (
	"context"
	"testing"
)

// Mock implements just UserGetter -- the interface the handler needs
type mockUserGetter struct {
	user *User
	err  error
}

func (m *mockUserGetter) Get(ctx context.Context, id string) (*User, error) {
	return m.user, m.err
}

func TestGetUserHandler_Success(t *testing.T) {
	mock := &mockUserGetter{
		user: &User{ID: "usr_123", Name: "Alice", Email: "alice@example.com"},
	}

	handler := NewGetUserHandler(mock)

	req := httptest.NewRequest("GET", "/users/usr_123", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestGetUserHandler_NotFound(t *testing.T) {
	mock := &mockUserGetter{
		err: ErrNotFound,
	}

	handler := NewGetUserHandler(mock)

	req := httptest.NewRequest("GET", "/users/usr_999", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}
```

No mocking framework needed. No code generation. The mock is 8 lines. This is only possible because the interface is small (one method) and defined at the consumer side.

## Verb+er Naming for Single-Method Interfaces

```go
// Standard library examples -- the pattern to follow
type Reader interface {
	Read(p []byte) (n int, err error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
}

type Closer interface {
	Close() error
}

type Stringer interface {
	String() string
}

// Application examples -- same pattern
type Validator interface {
	Validate() error
}

type Formatter interface {
	Format(data any) (string, error)
}

type Notifier interface {
	Notify(ctx context.Context, event Event) error
}

type Processor interface {
	Process(ctx context.Context, job Job) error
}

type Authenticator interface {
	Authenticate(ctx context.Context, token string) (*User, error)
}

type Serializer interface {
	Serialize(v any) ([]byte, error)
}
```

**When the Verb+er pattern does not apply** (multi-method interfaces), use a descriptive noun:

```go
// Multi-method interfaces use descriptive names
type UserRepository interface {
	Get(ctx context.Context, id string) (*User, error)
	Save(ctx context.Context, user *User) error
}

type EventPublisher interface {
	Publish(ctx context.Context, event Event) error
	Subscribe(ctx context.Context, topic string, handler EventHandler) error
}
```

## Key Points

- Go interfaces are satisfied implicitly -- the implementation never imports or references the interface
- Define interfaces at the consumer side, where they are used, not at the implementation side
- Return concrete structs from constructors -- the caller decides which interface the struct satisfies based on how it uses it
- Keep interfaces to 1-3 methods -- the smaller the interface, the more implementations can satisfy it
- Single-method interfaces use Verb+er naming (`Reader`, `Writer`, `Validator`)
- Compose small interfaces (`ReadWriter = Reader + Writer`) instead of defining large ones
- Small interfaces make testing trivial -- mocks are a few lines, no framework needed
- The standard library is the best reference: `io.Reader`, `io.Writer`, `fmt.Stringer`, `sort.Interface` -- all small, all consumer-side, all composable