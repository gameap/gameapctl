package oscore

import "strings"

// Well-known Windows account names in the exact form required by the service control manager.
//
// The security subsystem localizes these names, but the SCM does not support localized ones:
// CreateService and ChangeServiceConfig accept only the names below, regardless of the system
// locale. See https://learn.microsoft.com/en-us/windows/win32/services/networkservice-account.
const (
	WindowsNetworkServiceAccount = `NT AUTHORITY\NetworkService`
	WindowsLocalServiceAccount   = `NT AUTHORITY\LocalService`
	WindowsLocalSystemAccount    = `LocalSystem`
)

const windowsBuiltinAuthority = "NT AUTHORITY"

type windowsWellKnownAccount struct {
	serviceControlName string
	sid                string
}

var windowsWellKnownAccounts = map[string]windowsWellKnownAccount{
	"NETWORKSERVICE": {WindowsNetworkServiceAccount, "S-1-5-20"},
	"LOCALSERVICE":   {WindowsLocalServiceAccount, "S-1-5-19"},
	"LOCALSYSTEM":    {WindowsLocalSystemAccount, "S-1-5-18"},
	"SYSTEM":         {WindowsLocalSystemAccount, "S-1-5-18"},
}

// NormalizeWindowsServiceAccount returns the account name in the form accepted by the Windows
// service control manager. Display and localized spellings of the well-known service accounts
// are converted, everything else is returned unchanged.
func NormalizeWindowsServiceAccount(userName string) string {
	account, found := lookupWindowsWellKnownAccount(userName)
	if !found {
		return userName
	}

	return account.serviceControlName
}

// WindowsAccountIdentifier converts well-known account names to their SIDs in the form
// understood by icacls. Using SIDs ensures compatibility across different Windows locales
// and versions.
func WindowsAccountIdentifier(userName string) string {
	account, found := lookupWindowsWellKnownAccount(userName)
	if !found {
		return userName
	}

	return "*" + account.sid
}

func lookupWindowsWellKnownAccount(userName string) (windowsWellKnownAccount, bool) {
	name := strings.Trim(strings.TrimSpace(userName), `"`)

	if authority, account, found := strings.Cut(name, `\`); found {
		if !strings.EqualFold(strings.TrimSpace(authority), windowsBuiltinAuthority) {
			return windowsWellKnownAccount{}, false
		}

		name = account
	}

	key := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(name), " ", ""))

	account, found := windowsWellKnownAccounts[key]

	return account, found
}
