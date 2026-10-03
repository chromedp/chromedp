# Error Wrapping Patterns in Go

Complete examples showing proper error wrapping with `%w`, error message conventions, sentinel errors, custom error types, and how `errors.Is` / `errors.As` work with a properly wrapped error chain.

## Basic Error Wrapping

### Bad -- Error Chain Destroyed

```go
package order

import (
	"database/sql"
	"fmt"
)

func GetOrder(ctx context.Context, id string) (*Order, error) {
	row := db.QueryRowContext(ctx, "SELECT * FROM orders WHERE id = $1", id)

	var order Order
	err := row.Scan(&order.ID, &order.Total, &order.Status)
	if err != nil {
		// %s destroys the error chain -- errors.Is(err, sql.ErrNoRows) will fail
		return nil, fmt.Errorf("failed to get order %s: %s", id, err.Error())
	}

	return &order, nil
}

func GetOrderItems(ctx context.Context, orderID string) ([]Item, error) {
	rows, err := db.QueryContext(ctx, "SELECT * FROM items WHERE order_id = $1", orderID)
	if err != nil {
		// %v also destroys the chain
		return nil, fmt.Errorf("error fetching items: %v", err)
	}
	defer rows.Close()

	// ...
	return items, nil
}
```

### Good -- Error Chain Preserved

```go
package order

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func GetOrder(ctx context.Context, id string) (*Order, error) {
	row := db.QueryRowContext(ctx, "SELECT * FROM orders WHERE id = $1", id)

	var order Order
	err := row.Scan(&order.ID, &order.Total, &order.Status)
	if err != nil {
		// %w preserves the chain -- errors.Is(err, sql.ErrNoRows) works
		return nil, fmt.Errorf("getting order %s: %w", id, err)
	}

	return &order, nil
}

func GetOrderItems(ctx context.Context, orderID string) ([]Item, error) {
	rows, err := db.QueryContext(ctx, "SELECT * FROM items WHERE order_id = $1", orderID)
	if err != nil {
		return nil, fmt.Errorf("querying items for order %s: %w", orderID, err)
	}
	defer rows.Close()

	var items []Item
	for rows.Next() {
		var item Item
		if err := rows.Scan(&item.ID, &item.Name, &item.Price); err != nil {
			return nil, fmt.Errorf("scanning item row for order %s: %w", orderID, err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating items for order %s: %w", orderID, err)
	}

	return items, nil
}
```

## Error Message Conventions

```go
// BAD -- verbose, redundant, inconsistent
return nil, fmt.Errorf("Failed to connect to database: %w", err)         // capitalized
return nil, fmt.Errorf("error: could not parse config file: %w", err)     // "error:" is redundant
return nil, fmt.Errorf("an error occurred while fetching user: %w", err)  // too verbose
return nil, fmt.Errorf("Error in ProcessOrder: %w", err)                  // function name in error

// GOOD -- terse, lowercase, context-rich
return nil, fmt.Errorf("connecting to database at %s: %w", addr, err)
return nil, fmt.Errorf("parsing config %s: %w", path, err)
return nil, fmt.Errorf("fetching user %s: %w", userID, err)
return nil, fmt.Errorf("processing order %s: %w", orderID, err)
```

**Rules:**
- Start with a lowercase gerund: "getting", "parsing", "connecting", "validating"
- No "failed to" -- the error context already implies failure
- No "error:" prefix -- it is already an error
- No function names -- the stack trace has that
- Include identifying information: IDs, file paths, URLs, counts

## Sentinel Errors

```go
package order

import "errors"

// Sentinel errors -- exported, Err prefix, PascalCase
var (
	ErrNotFound      = errors.New("order not found")
	ErrAlreadyPaid   = errors.New("order already paid")
	ErrInvalidStatus = errors.New("invalid order status transition")
	ErrExpired       = errors.New("order expired")
)

func GetOrder(ctx context.Context, id string) (*Order, error) {
	order, err := db.QueryOrder(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound  // wrap with domain error
	}
	if err != nil {
		return nil, fmt.Errorf("querying order %s: %w", id, err)
	}
	return order, nil
}

func PayOrder(ctx context.Context, id string) error {
	order, err := GetOrder(ctx, id)
	if err != nil {
		return fmt.Errorf("paying order %s: %w", id, err)
	}

	if order.Status == StatusPaid {
		return ErrAlreadyPaid
	}

	if order.ExpiresAt.Before(time.Now()) {
		return ErrExpired
	}

	// process payment...
	return nil
}
```

```go
// Caller uses errors.Is to check sentinel errors through the chain
err := PayOrder(ctx, "ord_123")
if errors.Is(err, order.ErrNotFound) {
	http.Error(w, "Order not found", http.StatusNotFound)
	return
}
if errors.Is(err, order.ErrAlreadyPaid) {
	http.Error(w, "Order already paid", http.StatusConflict)
	return
}
if errors.Is(err, order.ErrExpired) {
	http.Error(w, "Order expired", http.StatusGone)
	return
}
if err != nil {
	slog.Error("failed to pay order", "error", err)
	http.Error(w, "Internal error", http.StatusInternalServerError)
	return
}
```

## Custom Error Types

When an error needs to carry structured data (not just a message), define a custom error type.

```go
package validation

import (
	"fmt"
	"strings"
)

// ErrValidation carries field-level error details.
type ErrValidation struct {
	Field   string
	Message string
	Value   any
}

func (e *ErrValidation) Error() string {
	return fmt.Sprintf("validation error on %s: %s (got %v)", e.Field, e.Message, e.Value)
}

// ErrMultiValidation aggregates multiple validation errors.
type ErrMultiValidation struct {
	Errors []*ErrValidation
}

func (e *ErrMultiValidation) Error() string {
	msgs := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		msgs[i] = err.Error()
	}
	return fmt.Sprintf("%d validation errors: %s", len(e.Errors), strings.Join(msgs, "; "))
}

// Usage
func ValidateOrder(order *Order) error {
	var errs []*ErrValidation

	if order.Total <= 0 {
		errs = append(errs, &ErrValidation{
			Field:   "total",
			Message: "must be positive",
			Value:   order.Total,
		})
	}

	if order.CustomerID == "" {
		errs = append(errs, &ErrValidation{
			Field:   "customer_id",
			Message: "is required",
			Value:   order.CustomerID,
		})
	}

	if len(errs) > 0 {
		return &ErrMultiValidation{Errors: errs}
	}

	return nil
}
```

```go
// Caller uses errors.As to extract the custom error type
err := ValidateOrder(order)
if err != nil {
	var multiErr *validation.ErrMultiValidation
	if errors.As(err, &multiErr) {
		for _, ve := range multiErr.Errors {
			fmt.Printf("Field %s: %s\n", ve.Field, ve.Message)
		}
		return
	}
	// not a validation error -- something else went wrong
	slog.Error("unexpected error validating order", "error", err)
}
```

## Error Wrapping Through Layers

A properly wrapped error chain reads like a stack trace in reverse, showing the path from the highest-level operation down to the root cause.

```go
// Layer 1: database
func (r *PostgresOrderRepo) GetByID(ctx context.Context, id string) (*Order, error) {
	// ...
	return nil, fmt.Errorf("querying order %s: %w", id, err)
	// Error: "querying order ord_123: connection refused"
}

// Layer 2: service
func (s *OrderService) GetOrder(ctx context.Context, id string) (*Order, error) {
	order, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting order from repo: %w", err)
	}
	// Error: "getting order from repo: querying order ord_123: connection refused"
	return order, nil
}

// Layer 3: handler
func (h *OrderHandler) HandleGetOrder(w http.ResponseWriter, r *http.Request) {
	order, err := h.service.GetOrder(r.Context(), id)
	if err != nil {
		slog.Error("handling get order request", "order_id", id, "error", err)
		// Log: "handling get order request order_id=ord_123
		//        error=getting order from repo: querying order ord_123: connection refused"
	}
}
```

Each layer adds context about what it was trying to do. The full chain tells the complete story: the handler was handling a get request, the service was getting from the repo, the repo was querying, and the root cause was a connection refused.

## Key Points

- `%w` preserves the error chain; `%s` and `%v` destroy it -- this is the single most common Go error-handling mistake
- Error messages are lowercase, start with a gerund, include IDs, and do not include "failed to" or function names
- Sentinel errors (`ErrNotFound`) are checked with `errors.Is()`; custom error types (`*ErrValidation`) are extracted with `errors.As()`
- Every layer in the call stack wraps with additional context -- the full chain reads as a narrative from high level to root cause
- `errorlint` in golangci-lint catches `%s`/`%v` on errors and `==` instead of `errors.Is()` automatically