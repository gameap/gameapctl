package packagemanager

import osinfo "github.com/gameap/gameapctl/pkg/os_info"

type distVersionPackagesMap map[osinfo.Distribution]map[string]map[string]map[string][]string

var aptPackageAliases = distVersionPackagesMap{
	DistributionDefault: {
		CodeNameDefault: {
			ArchDefault: {
				Lib32GCCPackage: {},
			},
		},
	},
	DistributionDebian: {
		CodeNameDefault: {
			ArchDefault: {
				Lib32GCCPackage: {},
			},
		},
		"squeeze": {
			ArchDefault: {
				MySQLServerPackage: {"mysql-server"},
				Lib32GCCPackage:    {"lib32gcc1"},
			},
		},
		"wheezy": {
			ArchDefault: {
				MySQLServerPackage: {"mysql-server"},
				Lib32GCCPackage:    {"lib32gcc1"},
			},
		},
		"jessie": {
			ArchDefault: {
				MySQLServerPackage: {"mysql-server"},
				Lib32GCCPackage:    {"lib32gcc1"},
			},
		},
		"stretch": {
			ArchDefault: {
				MySQLServerPackage: {"default-mysql-server"},
				Lib32GCCPackage:    {"lib32gcc1"},
			},
		},
		"buster": {
			ArchDefault: {
				MySQLServerPackage: {"default-mysql-server"},
				Lib32GCCPackage:    {"lib32gcc1"},
			},
		},
		"bullseye": {
			ArchDefault: {
				MySQLServerPackage: {"default-mysql-server"},
				Lib32GCCPackage:    {"lib32gcc-s1"},
			},
		},
		"bookworm": {
			ArchDefault: {
				MySQLServerPackage: {"default-mysql-server"},
				Lib32GCCPackage:    {"lib32gcc-s1"},
				NodeJSPackage:      {"nodejs", "npm"},
			},
			ArchARM64: {
				Lib32z1Package:    {""},
				Lib32GCCPackage:   {"lib32gcc-s1-amd64-cross"},
				Lib32Stdc6Package: {"lib32stdc++6-amd64-cross"},
				NodeJSPackage:     {"nodejs", "npm"},
			},
		},
		"sid": {
			ArchDefault: {
				MySQLServerPackage: {"default-mysql-server"},
				Lib32GCCPackage:    {"lib32gcc-s1"},
			},
			ArchARM64: {
				Lib32z1Package:    {""},
				Lib32GCCPackage:   {"lib32gcc-s1-amd64-cross"},
				Lib32Stdc6Package: {"lib32stdc++6-amd64-cross"},
			},
		},
	},
	DistributionUbuntu: {
		CodeNameDefault: {
			ArchDefault: {
				Lib32GCCPackage: {},
			},
		},
		"precise": {
			ArchDefault: {
				Lib32GCCPackage: {"lib32gcc1"},
			},
		},
		"trusty": {
			ArchDefault: {
				Lib32GCCPackage: {"lib32gcc1"},
			},
		},
		"xenial": {
			ArchDefault: {
				Lib32GCCPackage: {"lib32gcc1"},
			},
			ArchARM64: {
				Lib32GCCPackage: {"lib32gcc-s1-amd64-cross"},
			},
		},
		"bionic": {
			ArchDefault: {
				Lib32GCCPackage: {"lib32gcc1"},
			},
			ArchARM64: {
				Lib32GCCPackage: {"lib32gcc-s1-amd64-cross"},
			},
		},
		"focal": {
			ArchDefault: {
				Lib32GCCPackage: {"lib32gcc1"},
			},
			ArchARM64: {
				Lib32GCCPackage: {"lib32gcc-s1-amd64-cross"},
			},
		},
		"jammy": {
			ArchDefault: {
				Lib32GCCPackage: {"lib32gcc-s1"},
			},
			ArchARM64: {
				Lib32GCCPackage: {"lib32gcc-s1-amd64-cross"},
			},
		},
		"kinetic": {
			ArchDefault: {
				Lib32GCCPackage: {"lib32gcc-s1"},
			},
			ArchARM64: {
				Lib32GCCPackage: {"lib32gcc-s1-amd64-cross"},
			},
		},
		"lunar": {
			ArchDefault: {
				Lib32GCCPackage: {"lib32gcc-s1"},
			},
			ArchARM64: {
				Lib32GCCPackage: {"lib32gcc-s1-amd64-cross"},
			},
		},
		"mantic": {
			ArchDefault: {
				Lib32GCCPackage:      {"lib32gcc-s1"},
				PHPExtensionsPackage: {""},
			},
			ArchARM64: {
				Lib32GCCPackage: {"lib32gcc-s1-amd64-cross"},
			},
		},
	},
}
