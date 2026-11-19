package accounts

import "testing"

func TestUsernames(t *testing.T) {
	names := Usernames(2000, 5)
	if len(names) != 2005 {
		t.Fatalf("len = %d, want 2005", len(names))
	}
	if names[0] != "loadbuyer0001" || names[1999] != "loadbuyer2000" || names[2000] != "loadscalper01" || names[2004] != "loadscalper05" {
		t.Fatalf("unexpected names: %s %s %s %s", names[0], names[1999], names[2000], names[2004])
	}
	seen := map[string]bool{}
	for _, n := range names {
		if seen[n] {
			t.Fatalf("duplicate %s", n)
		}
		seen[n] = true
	}
}

func TestBuildSetsPasswordAndEmail(t *testing.T) {
	users := Build(1, 1, "secret")
	if len(users) != 2 {
		t.Fatalf("len = %d", len(users))
	}
	u := users[0]
	if !u.Enabled || u.Email != "loadbuyer0001@load.ticketrush.test" || len(u.Credentials) != 1 ||
		u.Credentials[0].Value != "secret" || u.Credentials[0].Temporary ||
		len(u.RealmRoles) != 1 || u.RealmRoles[0] != "buyer" || len(users[1].RealmRoles) != 1 {
		t.Fatalf("user = %+v", u)
	}
}
