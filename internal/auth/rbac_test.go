package auth

import "testing"

func TestHasPermission(t *testing.T) {
	tests := []struct {
		role   Role
		action Action
		want   bool
	}{
		{RoleAdmin, ActionAdmin, true},
		{RoleAdmin, ActionRead, true},
		{RoleUser, ActionWrite, true},
		{RoleUser, ActionAdmin, false},
		{RoleGuest, ActionRead, true},
		{RoleGuest, ActionWrite, false},
		{RoleGuest, ActionTransfer, false},
	}
	for _, tt := range tests {
		got := HasPermission(tt.role, tt.action)
		if got != tt.want {
			t.Errorf("HasPermission(%s, %s) = %v, want %v", tt.role, tt.action, got, tt.want)
		}
	}
}

func TestCanAccessNode(t *testing.T) {
	if !CanAccessNode(RoleUser, false) {
		t.Fatal("user should access writable node")
	}
	if !CanAccessNode(RoleGuest, true) {
		t.Fatal("guest should access read-only node")
	}
	if CanAccessNode(RoleGuest, false) {
		t.Fatal("guest should not access writable node")
	}
}

func TestRoleRank(t *testing.T) {
	if RoleRank(RoleAdmin) <= RoleRank(RoleUser) {
		t.Fatal("admin should outrank user")
	}
	if RoleRank(RoleUser) <= RoleRank(RoleGuest) {
		t.Fatal("user should outrank guest")
	}
}

func TestMinRole(t *testing.T) {
	if MinRole(ActionAdmin) != RoleAdmin {
		t.Fatal("admin action requires admin role")
	}
	if MinRole(ActionWrite) != RoleUser {
		t.Fatal("write action requires user role")
	}
	if MinRole(ActionRead) != RoleGuest {
		t.Fatal("read action requires guest role")
	}
}
