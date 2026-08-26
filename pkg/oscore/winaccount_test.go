package oscore

import "testing"

func TestNormalizeWindowsServiceAccount(t *testing.T) {
	tests := []struct {
		name     string
		userName string
		expected string
	}{
		{
			name:     "english display name of network service",
			userName: `NT AUTHORITY\NETWORK SERVICE`,
			expected: `NT AUTHORITY\NetworkService`,
		},
		{
			name:     "canonical name of network service is kept",
			userName: `NT AUTHORITY\NetworkService`,
			expected: `NT AUTHORITY\NetworkService`,
		},
		{
			name:     "network service without authority",
			userName: "Network Service",
			expected: `NT AUTHORITY\NetworkService`,
		},
		{
			name:     "quoted and padded name",
			userName: `  "nt authority\network service"  `,
			expected: `NT AUTHORITY\NetworkService`,
		},
		{
			name:     "local service",
			userName: `NT AUTHORITY\LOCAL SERVICE`,
			expected: `NT AUTHORITY\LocalService`,
		},
		{
			name:     "system is local system",
			userName: `NT AUTHORITY\SYSTEM`,
			expected: "LocalSystem",
		},
		{
			name:     "local system",
			userName: "LocalSystem",
			expected: "LocalSystem",
		},
		{
			name:     "domain account is not touched",
			userName: `CORP\NetworkService`,
			expected: `CORP\NetworkService`,
		},
		{
			name:     "regular user is not touched",
			userName: "gameap",
			expected: "gameap",
		},
		{
			name:     "empty name",
			userName: "",
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := NormalizeWindowsServiceAccount(test.userName)

			if actual != test.expected {
				t.Errorf("expected %q, got %q", test.expected, actual)
			}
		})
	}
}

func TestWindowsAccountIdentifier(t *testing.T) {
	tests := []struct {
		name     string
		userName string
		expected string
	}{
		{
			name:     "english display name of network service",
			userName: `NT AUTHORITY\NETWORK SERVICE`,
			expected: "*S-1-5-20",
		},
		{
			name:     "canonical name of network service",
			userName: `NT AUTHORITY\NetworkService`,
			expected: "*S-1-5-20",
		},
		{
			name:     "local service",
			userName: `NT AUTHORITY\LOCAL SERVICE`,
			expected: "*S-1-5-19",
		},
		{
			name:     "system",
			userName: `NT AUTHORITY\SYSTEM`,
			expected: "*S-1-5-18",
		},
		{
			name:     "local system",
			userName: "LocalSystem",
			expected: "*S-1-5-18",
		},
		{
			name:     "domain account is not touched",
			userName: `CORP\SYSTEM`,
			expected: `CORP\SYSTEM`,
		},
		{
			name:     "regular user is not touched",
			userName: "gameap",
			expected: "gameap",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := WindowsAccountIdentifier(test.userName)

			if actual != test.expected {
				t.Errorf("expected %q, got %q", test.expected, actual)
			}
		})
	}
}
