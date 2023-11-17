package packagemanager

var dnfPackageAliases = distVersionPackagesMap{
	Default: {
		Default: {
			ArchDefault: {
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
	DistributionCentOS: {
		Default: {
			ArchDefault: {
				Lib32GCCPackage:   {"libgcc.i686"},
				Lib32Stdc6Package: {"libstdc++.i686"},
				Lib32z1Package:    {"zlib.i686"},
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
}
