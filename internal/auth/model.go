package auth

import "time"

type Role string

const (
	RoleOwner  Role = "owner"
	RoleMember Role = "member"
)

type TokenPurpose string

const (
	TokenPurposeVerifyEmail   TokenPurpose = "verify_email"
	TokenPurposeResetPassword TokenPurpose = "reset_password"
)

type User struct {
	ID              int64
	Email           string
	EmailVerifiedAt *time.Time
	DisabledAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type Credential struct {
	User         User
	PasswordHash string
}

func (u User) EmailVerified() bool {
	return u.EmailVerifiedAt != nil
}

type Tenant struct {
	ID        int64
	Name      string
	IsLegacy  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Membership struct {
	TenantID  int64
	UserID    int64
	Role      Role
	CreatedAt time.Time
}

type Principal struct {
	User       User
	Tenant     Tenant
	Membership Membership
}

func (p Principal) EmailVerified() bool {
	return p.User.EmailVerified()
}

type AccountToken struct {
	ID          int64
	UserID      int64
	Purpose     TokenPurpose
	TargetEmail string
	ExpiresAt   time.Time
	ConsumedAt  *time.Time
	CreatedAt   time.Time
}

type AuthResult struct {
	Principal         Principal
	SessionToken      string
	SessionExpiresAt  time.Time
	VerificationToken string
}
