// Package accounts names the load-test accounts, so the tool that creates them and the bots that use them agree.
package accounts

import (
	"fmt"

	"github.com/johancarloss/ticket-rush/bots/internal/keycloak"
)

const (
	BuyerPrefix   = "loadbuyer"
	ScalperPrefix = "loadscalper"
	emailDomain   = "load.ticketrush.test"
)

// Usernames lists the fans first and then the scalpers: loadbuyer0001..., loadscalper01...
func Usernames(buyers, scalpers int) []string {
	names := make([]string, 0, buyers+scalpers)
	for i := 1; i <= buyers; i++ {
		names = append(names, fmt.Sprintf("%s%04d", BuyerPrefix, i))
	}
	for i := 1; i <= scalpers; i++ {
		names = append(names, fmt.Sprintf("%s%02d", ScalperPrefix, i))
	}
	return names
}

// Build returns the accounts to import, all sharing one password from the environment.
// The partial import does not apply the realm's default roles (a signup or the admin "create user" would), so the
// buyer role is set on each account, as in the users of the realm file. Scalpers are buyers to the system too.
func Build(buyers, scalpers int, password string) []keycloak.User {
	names := Usernames(buyers, scalpers)
	users := make([]keycloak.User, len(names))
	for i, name := range names {
		users[i] = keycloak.User{
			Username:      name,
			Email:         name + "@" + emailDomain,
			FirstName:     "Load",
			LastName:      name,
			Enabled:       true,
			EmailVerified: true,
			RealmRoles:    []string{"buyer"},
			Credentials:   []keycloak.Credential{{Type: "password", Value: password, Temporary: false}},
		}
	}
	return users
}
