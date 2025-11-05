package packagemanager

var dnfPackageAliases = distVersionPackagesMap{
	DistributionDefault: {
		CodeNameDefault: {
			ArchDefault: {
				Lib32GCCPackage:   {"libgcc.i686"},
				Lib32Stdc6Package: {"libstdc++.i686"},
				Lib32z1Package:    {"zlib.i686"},
				XZUtilsPackage:    {"xz"},
				PHPPackage:        {"php-cli", "php-common", "php-fpm"},
				PHPExtensionsPackage: {
					"php-bcmath", "php-gd", "php-gmp", "php-intl",
					"php-json", "php-mbstring", "php-mysqlnd", "php-opcache",
					"php-openssl", "php-pdo", "php-pdo", "php-pecl-zip",
					"php-readline", "php-session", "php-sockets", "php-tokenizer",
					"php-xml", "php-zip",
				},
				PostgreSQLPackage: {"postgresql-server", "postgresql-contrib"},
			},
		},
	},
	DistributionCentOS: {
		CodeNameDefault: {
			ArchDefault: {
				Lib32GCCPackage:   {"libgcc.i686"},
				Lib32Stdc6Package: {"libstdc++.i686"},
				Lib32z1Package:    {"zlib.i686"},
				XZUtilsPackage:    {"xz"},
				PHPPackage:        {"php-cli", "php-common", "php-fpm"},
				PHPExtensionsPackage: {
					"php-bcmath", "php-gd", "php-gmp", "php-intl",
					"php-json", "php-mbstring", "php-mysqlnd", "php-opcache",
					"php-openssl", "php-pdo", "php-pdo", "php-pecl-zip",
					"php-readline", "php-session", "php-sockets", "php-tokenizer",
					"php-xml", "php-zip",
				},
			},
		},
	},
	DistributionAmazon: {
		CodeNameDefault: {
			ArchDefault: {
				Lib32GCCPackage:      {"libgcc.i686"},
				Lib32Stdc6Package:    {"libstdc++.i686"},
				Lib32z1Package:       {"zlib.i686"},
				XZUtilsPackage:       {"xz"},
				PHPPackage:           {"php-cli", "php-common", "php-fpm"},
				MariaDBServerPackage: {"mariadb105"},
				PHPExtensionsPackage: {
					"php-bcmath", "php-gd", "php-gmp", "php-intl",
					"php-json", "php-mbstring", "php-mysqlnd", "php-opcache",
					"php-openssl", "php-pdo", "php-pdo", "php-zip",
					"php-readline", "php-session", "php-sockets", "php-tokenizer",
					"php-xml", "php-zip",
				},
			},
		},
	},
	DistributionRocky: {
		CodeNameDefault: {
			ArchDefault: {
				Lib32GCCPackage:   {""},
				Lib32Stdc6Package: {""},
				Lib32z1Package:    {""},
			},
		},
	},
}
