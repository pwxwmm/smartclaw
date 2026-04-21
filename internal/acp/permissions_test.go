package acp

import (
	"testing"
)

func TestPermissionModel_AddRuleAndCheck(t *testing.T) {
	pm := NewPermissionModel()

	pm.AddRule(PermissionRule{Resource: "tools:*", Level: PermissionExecute, Allowed: true})
	pm.AddRule(PermissionRule{Resource: "files:read", Level: PermissionRead, Allowed: true})
	pm.AddRule(PermissionRule{Resource: "files:write", Level: PermissionWrite, Allowed: false})

	if !pm.Check("tools:bash", PermissionExecute) {
		t.Error("tools:bash execute should be allowed")
	}
	if !pm.Check("files:read", PermissionRead) {
		t.Error("files:read read should be allowed")
	}
	if pm.Check("files:write", PermissionWrite) {
		t.Error("files:write write should be denied")
	}
}

func TestPermissionModel_RemoveRule(t *testing.T) {
	pm := NewPermissionModel()
	pm.AddRule(PermissionRule{Resource: "sessions:own", Level: PermissionWrite, Allowed: true})

	if !pm.Check("sessions:own", PermissionWrite) {
		t.Error("should be allowed before remove")
	}

	pm.RemoveRule("sessions:own", PermissionWrite)
	if pm.Check("sessions:own", PermissionWrite) {
		t.Error("should be denied after remove")
	}
}

func TestPermissionModel_WildcardMatch(t *testing.T) {
	pm := NewPermissionModel()
	pm.AddRule(PermissionRule{Resource: "*:*", Level: PermissionRead, Allowed: true})

	if !pm.Check("tools:bash", PermissionRead) {
		t.Error("*:* should match tools:bash for read")
	}
	if !pm.Check("memory:store", PermissionRead) {
		t.Error("*:* should match memory:store for read")
	}
}

func TestPermissionModel_FirstMatchWins(t *testing.T) {
	pm := NewPermissionModel()
	pm.AddRule(PermissionRule{Resource: "files:*", Level: PermissionWrite, Allowed: false})
	pm.AddRule(PermissionRule{Resource: "files:tmp", Level: PermissionWrite, Allowed: true})

	if pm.Check("files:tmp", PermissionWrite) {
		t.Error("first matching rule (files:* deny) should win over later specific rule")
	}
}

func TestPermissionModel_NoMatchReturnsFalse(t *testing.T) {
	pm := NewPermissionModel()
	if pm.Check("anything", PermissionAdmin) {
		t.Error("no rules should deny by default")
	}
}

func TestPermissionModel_DefineRole(t *testing.T) {
	pm := NewPermissionModel()
	pm.AddRule(PermissionRule{Resource: "*:*", Level: PermissionRead, Allowed: true})

	pm.DefineRole("admin", []PermissionRule{
		{Resource: "*:*", Level: PermissionAdmin, Allowed: true},
		{Resource: "*:*", Level: PermissionWrite, Allowed: true},
	})

	if !pm.CheckRole("admin", "tools:bash", PermissionAdmin) {
		t.Error("admin role should allow admin on tools:bash")
	}
	if !pm.CheckRole("admin", "files:read", PermissionWrite) {
		t.Error("admin role should allow write on files:read")
	}
}

func TestPermissionModel_CheckRoleFallsBackToGlobal(t *testing.T) {
	pm := NewPermissionModel()
	pm.AddRule(PermissionRule{Resource: "*:*", Level: PermissionRead, Allowed: true})

	pm.DefineRole("viewer", []PermissionRule{})

	if !pm.CheckRole("viewer", "tools:list", PermissionRead) {
		t.Error("viewer should inherit global read permission")
	}
}

func TestPermissionModel_CheckRoleUnknownRole(t *testing.T) {
	pm := NewPermissionModel()
	pm.AddRule(PermissionRule{Resource: "tools:*", Level: PermissionExecute, Allowed: true})

	if !pm.CheckRole("nonexistent", "tools:bash", PermissionExecute) {
		t.Error("unknown role should fall back to global rules")
	}
}

func TestDefaultPermissions(t *testing.T) {
	defaults := DefaultPermissions()

	pm := NewPermissionModel()
	for _, r := range defaults {
		pm.AddRule(r)
	}

	if !pm.Check("tools:bash", PermissionRead) {
		t.Error("default: read all should allow reading tools")
	}
	if !pm.Check("sessions:own", PermissionWrite) {
		t.Error("default: write own sessions should be allowed")
	}
	if !pm.Check("tools:bash", PermissionExecute) {
		t.Error("default: execute tools should be allowed")
	}
	if pm.Check("anything", PermissionAdmin) {
		t.Error("default: admin should be denied for everything")
	}
}
