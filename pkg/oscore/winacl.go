package oscore

// Well-known SIDs of the accounts that keep access to a restricted directory. Unlike the
// account names, the SIDs are the same on every Windows locale.
const (
	windowsLocalSystemSID    = "S-1-5-18"
	windowsAdministratorsSID = "S-1-5-32-544"
	windowsUsersSID          = "S-1-5-32-545"
)

// restrictDirectoryAccessArgs returns the icacls arguments that give the directory a protected
// DACL: the inherited entries are removed, SYSTEM and Administrators get full control and Users
// get read and execute access, all of them inherited by files and subdirectories.
//
// /T is left out on purpose: with it icacls would disable inheritance on every file and
// subdirectory as well. Without it the children keep their explicit entries and Windows
// recomputes the inherited ones from the new DACL of the directory.
func restrictDirectoryAccessArgs(path string) []string {
	return []string{
		path,
		"/inheritance:r",
		"/grant:r",
		"*" + windowsLocalSystemSID + ":(OI)(CI)F",
		"*" + windowsAdministratorsSID + ":(OI)(CI)F",
		"*" + windowsUsersSID + ":(OI)(CI)RX",
	}
}

// restrictDirectoryOwnerArgs returns the icacls arguments that make Administrators the owner of
// the directory. The owner may always rewrite the DACL, so a directory that a regular user
// created before the installation would otherwise stay under that user's control.
func restrictDirectoryOwnerArgs(path string) []string {
	return []string{path, "/setowner", "*" + windowsAdministratorsSID}
}
