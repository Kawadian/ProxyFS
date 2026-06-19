package hub

import (
	"database/sql"
	"hash/fnv"
)

const (
	BaseUID = 10000
	BaseGID = 10000
	UIDSpan = 50000
)

// UIDGID returns deterministic POSIX IDs for a Hub user.
func UIDGID(userID string) (uint32, uint32) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(userID))
	offset := h.Sum32() % UIDSpan
	return BaseUID + offset, BaseGID + offset
}

// GuestUIDGID returns IDs for anonymous guest access.
func GuestUIDGID() (uint32, uint32) {
	return 65534, 65534
}

// LoadMapping resolves UID/GID from samba_accounts when present, otherwise derives them.
func LoadMapping(db *sql.DB, userID, username string) (uint32, uint32) {
	var uid, gid int
	err := db.QueryRow(
		`SELECT uid, gid FROM samba_accounts WHERE user_id = ?`,
		userID,
	).Scan(&uid, &gid)
	if err == nil {
		return uint32(uid), uint32(gid)
	}
	return UIDGID(userID)
}

// FromRoles builds a Hub user for RBAC checks.
func FromRoles(id, username string, roles []string, isGuest bool) *User {
	uid, gid := UIDGID(id)
	if isGuest {
		uid, gid = GuestUIDGID()
	}
	return &User{
		ID:       id,
		Username: username,
		UID:      uid,
		GID:      gid,
		Roles:    roles,
		IsGuest:  isGuest,
	}
}
