package oscore

type CreateUserOption func(o *createUserOptions)

// WithWorkDir allows to specify the working directory for the user.
func WithWorkDir(workDir string) CreateUserOption {
	return func(o *createUserOptions) {
		o.workDir = workDir
	}
}

func WithPassword(password string) CreateUserOption {
	return func(o *createUserOptions) {
		o.password = password
	}
}

func WithShell(shell string) CreateUserOption {
	return func(o *createUserOptions) {
		o.shell = shell
	}
}

type createUserOptions struct {
	workDir  string
	password string
	shell    string
}

func applyCreateUserOptions(opts ...CreateUserOption) *createUserOptions {
	o := &createUserOptions{}

	for _, opt := range opts {
		opt(o)
	}

	return o
}

type CreateGroupOption func(o *createGroupOptions)

// WithGID allows to specify the group ID for the group.
func WithGID(gid string) CreateGroupOption {
	return func(o *createGroupOptions) {
		o.gid = gid
	}
}

type createGroupOptions struct {
	gid string
}

//nolint:unused,nolintlint
func applyCreateGroupOptions(opts ...CreateGroupOption) *createGroupOptions {
	o := &createGroupOptions{}

	for _, opt := range opts {
		opt(o)
	}

	return o
}
