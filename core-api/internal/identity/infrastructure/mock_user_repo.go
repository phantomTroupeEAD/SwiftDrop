package infrastructure

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// MockUserRepository is a thread-safe in-memory implementation of application.UserRepository.
// It is preloaded with 5 mock accounts (one for each role) to enable immediate team-wide development.
type MockUserRepository struct {
	mu    sync.RWMutex
	users map[string]*domain.User // keyed by normalized email
	byID  map[string]*domain.User // keyed by user ID
}

// NewMockUserRepository initializes an in-memory repository pre-seeded with 5 role accounts.
func NewMockUserRepository(hasher application.PasswordHasher) *MockUserRepository {
	repo := &MockUserRepository{
		users: make(map[string]*domain.User),
		byID:  make(map[string]*domain.User),
	}

	merchantTenant := "mrc_01HMERCHANTTENANT00001"
	now := time.Now().UTC()

	// Seed 5 accounts (one for each role)
	seedAccounts := []struct {
		id          string
		email       string
		password    string
		displayName string
		phone       string
		role        domain.Role
		tenantID    *string
	}{
		{
			id:          "usr_01HADMIN000000000000000001",
			email:       "admin@swiftdrop.local",
			password:    "AdminPassword123!",
			displayName: "System Administrator",
			phone:       "+94770000001",
			role:        domain.RoleAdmin,
			tenantID:    nil,
		},
		{
			id:          "usr_01HDISPATCH00000000000002",
			email:       "dispatcher@swiftdrop.local",
			password:    "Dispatcher123!",
			displayName: "Central Dispatcher",
			phone:       "+94770000002",
			role:        domain.RoleDispatcher,
			tenantID:    nil,
		},
		{
			id:          "usr_01HCOURIER00000000000003",
			email:       "courier@swiftdrop.local",
			password:    "CourierPassword123!",
			displayName: "Kamal Perera (Courier)",
			phone:       "+94770000003",
			role:        domain.RoleCourier,
			tenantID:    nil,
		},
		{
			id:          "usr_01HMERCHANT00000000000004",
			email:       "merchant@vendor.com",
			password:    "Merchant123!",
			displayName: "Colombo Tech Stores",
			phone:       "+94770000004",
			role:        domain.RoleMerchant,
			tenantID:    &merchantTenant,
		},
		{
			id:          "usr_01HCUSTOMER00000000000005",
			email:       "customer@swiftdrop.local",
			password:    "CustomerPassword123!",
			displayName: "Nimal Fernando",
			phone:       "+94770000005",
			role:        domain.RoleCustomer,
			tenantID:    nil,
		},
	}

	for _, s := range seedAccounts {
		emailVO, _ := domain.NewEmail(s.email)
		hash, _ := hasher.HashPassword(s.password)

		u := domain.NewUser(
			s.id,
			emailVO,
			hash,
			s.displayName,
			s.phone,
			s.role,
			s.tenantID,
			domain.StatusActive,
			now,
			now,
		)
		repo.users[string(emailVO)] = u
		repo.byID[s.id] = u
	}

	return repo
}

// FindByEmail retrieves a user by their email address.
func (r *MockUserRepository) FindByEmail(_ context.Context, email string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalized := strings.ToLower(strings.TrimSpace(email))
	u, exists := r.users[normalized]
	if !exists {
		return nil, domain.ErrUserNotFound
	}

	// Return a copy to preserve immutability
	clone := *u
	return &clone, nil
}

// FindByID retrieves a user by their internal UUID.
func (r *MockUserRepository) FindByID(_ context.Context, id string) (*domain.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	u, exists := r.byID[id]
	if !exists {
		return nil, domain.ErrUserNotFound
	}

	clone := *u
	return &clone, nil
}

// Save inserts or updates a user aggregate.
func (r *MockUserRepository) Save(_ context.Context, user *domain.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	clone := *user
	r.users[string(user.Email)] = &clone
	r.byID[user.ID] = &clone
	return nil
}

// ListUsers returns paginated users with optional role filtering.
func (r *MockUserRepository) ListUsers(_ context.Context, role *domain.Role, limit int, offset int) ([]*domain.User, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var matched []*domain.User
	for _, u := range r.users {
		if role == nil || u.Role == *role {
			clone := *u
			matched = append(matched, &clone)
		}
	}

	total := len(matched)
	if offset >= total {
		return []*domain.User{}, total, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return matched[offset:end], total, nil
}
