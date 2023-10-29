package selfupdate

type FailedToGetLatestVersionError string

func (e FailedToGetLatestVersionError) Error() string {
	return string(e)
}
