package auth

// Role identifies a user's access tier.
type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
	RoleGuest Role = "guest"
)

// Action is an operation that can be authorized.
type Action string

const (
	ActionRead      Action = "read"
	ActionWrite     Action = "write"
	ActionDelete    Action = "delete"
	ActionAdmin     Action = "admin"
	ActionTransfer  Action = "transfer"
	ActionManageKey Action = "manage_key"
)

// Resource categorizes protected objects.
type Resource string

const (
	ResourceFile     Resource = "file"
	ResourceNode     Resource = "node"
	ResourceUser     Resource = "user"
	ResourceKey      Resource = "key"
	ResourceTransfer Resource = "transfer"
	ResourceSystem   Resource = "system"
)

var rolePermissions = map[Role]map[Action]bool{
	RoleAdmin: {
		ActionRead:      true,
		ActionWrite:     true,
		ActionDelete:    true,
		ActionAdmin:     true,
		ActionTransfer:  true,
		ActionManageKey: true,
	},
	RoleUser: {
		ActionRead:     true,
		ActionWrite:    true,
		ActionDelete:   true,
		ActionTransfer: true,
	},
	RoleGuest: {
		ActionRead: true,
	},
}

// HasPermission reports whether a role may perform an action.
func HasPermission(role Role, action Action) bool {
	perms, ok := rolePermissions[role]
	if !ok {
		return false
	}
	return perms[action]
}

// CanAccessNode checks node-level access for a role.
func CanAccessNode(role Role, readOnly bool) bool {
	if role == RoleAdmin || role == RoleUser {
		return true
	}
	if role == RoleGuest && readOnly {
		return true
	}
	return false
}

// RoleRank returns a numeric rank for role comparison (higher is more privileged).
func RoleRank(role Role) int {
	switch role {
	case RoleAdmin:
		return 3
	case RoleUser:
		return 2
	case RoleGuest:
		return 1
	default:
		return 0
	}
}

// MinRole returns the minimum role required for an action.
func MinRole(action Action) Role {
	switch action {
	case ActionAdmin, ActionManageKey:
		return RoleAdmin
	case ActionWrite, ActionDelete, ActionTransfer:
		return RoleUser
	default:
		return RoleGuest
	}
}
