package packagemanager

type distVersionPackagesMap map[string]map[string]map[string]map[string][]string

var packageAliases = distVersionPackagesMap{
	"debian": {
		"squeeze": {
			"default": {
				MySQLServerPackage: {"mysql-server"},
				Lib32GCCPackage:    {"lib32gcc1"},
			},
		},
		"wheezy": {
			"default": {
				MySQLServerPackage: {"mysql-server"},
				Lib32GCCPackage:    {"lib32gcc1"},
			},
		},
		"jessie": {
			"default": {
				MySQLServerPackage: {"mysql-server"},
				Lib32GCCPackage:    {"lib32gcc1"},
			},
		},
		"stretch": {
			"default": {
				MySQLServerPackage: {"default-mysql-server"},
				Lib32GCCPackage:    {"lib32gcc1"},
			},
		},
		"buster": {
			"default": {
				MySQLServerPackage: {"default-mysql-server"},
				Lib32GCCPackage:    {"lib32gcc1"},
			},
		},
		"bullseye": {
			"default": {
				MySQLServerPackage: {"default-mysql-server"},
				Lib32GCCPackage:    {"lib32gcc-s1"},
			},
		},
		"bookworm": {
			"default": {
				MySQLServerPackage: {"default-mysql-server"},
				Lib32GCCPackage:    {"lib32gcc-s1"},
			},
			"arm64": {
				Lib32z1Package:    {""},
				Lib32GCCPackage:   {"lib32gcc-s1-amd64-cross"},
				Lib32Stdc6Package: {"lib32stdc++6-amd64-cross"},
			},
		},
		"sid": {
			"default": {
				MySQLServerPackage: {"default-mysql-server"},
				Lib32GCCPackage:    {"lib32gcc-s1"},
			},
			"arm64": {
				Lib32z1Package:    {""},
				Lib32GCCPackage:   {"lib32gcc-s1-amd64-cross"},
				Lib32Stdc6Package: {"lib32stdc++6-amd64-cross"},
			},
		},
	},
	"ubuntu": {
		"precise": {
			"default": {
				Lib32GCCPackage: {"lib32gcc1"},
			},
		},
		"trusty": {
			"default": {
				Lib32GCCPackage: {"lib32gcc1"},
			},
		},
		"xenial": {
			"default": {
				Lib32GCCPackage: {"lib32gcc1"},
			},
			"arm64": {
				Lib32GCCPackage: {"lib32gcc-s1-amd64-cross"},
			},
		},
		"bionic": {
			"default": {
				Lib32GCCPackage: {"lib32gcc1"},
			},
			"arm64": {
				Lib32GCCPackage: {"lib32gcc-s1-amd64-cross"},
			},
		},
		"focal": {
			"default": {
				Lib32GCCPackage: {"lib32gcc1"},
			},
			"arm64": {
				Lib32GCCPackage: {"lib32gcc-s1-amd64-cross"},
			},
		},
		"jammy": {
			"default": {
				Lib32GCCPackage: {"lib32gcc-s1"},
			},
			"arm64": {
				Lib32GCCPackage: {"lib32gcc-s1-amd64-cross"},
			},
		},
		"kinetic": {
			"default": {
				Lib32GCCPackage: {"lib32gcc-s1"},
			},
			"arm64": {
				Lib32GCCPackage: {"lib32gcc-s1-amd64-cross"},
			},
		},
		"lunar": {
			"default": {
				Lib32GCCPackage: {"lib32gcc-s1"},
			},
			"arm64": {
				Lib32GCCPackage: {"lib32gcc-s1-amd64-cross"},
			},
		},
	},
}
