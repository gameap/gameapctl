package packagemanager

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parseYumInfoOutput(t *testing.T) {
	out := []byte(`
Last metadata expiration check: 0:09:02 ago on Fri 17 Nov 2023 04:11:17 AM UTC.
Available Packages
Name         : mysql
Version      : 8.0.32
Release      : 1.el9
Architecture : x86_64
Size         : 2.8 M
Source       : mysql-8.0.32-1.el9.src.rpm
Repository   : appstream
Summary      : MySQL client programs and shared libraries
URL          : http://www.mysql.com
License      : GPLv2 with exceptions and LGPLv2 and BSD
Description  : MySQL is a multi-user, multi-threaded SQL database server. MySQL is a
             : client/server implementation consisting of a server daemon (mysqld)
             : and many different client programs and libraries. The base package
             : contains the standard MySQL client programs and generic MySQL files.

Name         : mysql-common
Version      : 8.0.32
Release      : 1.el9
Architecture : x86_64
Size         : 75 k
Source       : mysql-8.0.32-1.el9.src.rpm
Repository   : appstream
Summary      : The shared files required for MySQL server and client
URL          : http://www.mysql.com
License      : GPLv2 with exceptions and LGPLv2 and BSD
Description  : The mysql-common package provides the essential shared files for any
             : MySQL program. You will need to install this package to use any other
             : MySQL package.

	`)
	parsed, err := parseYumInfoOutput(out)

	require.NoError(t, err)
	require.Equal(t, []PackageInfo{
		{
			Name:         "mysql",
			Version:      "8.0.32",
			Architecture: "x86_64",
			Size:         "2.8 M",
			Description: "MySQL is a multi-user, multi-threaded SQL database server. " +
				"MySQL is a client/server implementation consisting of a server daemon (mysqld)" +
				" and many different client programs and libraries. The base package contains the " +
				"standard MySQL client programs and generic MySQL files.",
		},
		{
			Name:         "mysql-common",
			Version:      "8.0.32",
			Architecture: "x86_64",
			Size:         "75 k",
			Description: "The mysql-common package provides the essential shared files for any " +
				"MySQL program. You will need to install this package to use any other MySQL package.",
		},
	}, parsed)
}
