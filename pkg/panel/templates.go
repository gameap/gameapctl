package panel

const configEnvTemplate = `# Server
HTTP_HOST={{.HTTPHost}}
HTTP_PORT={{.HTTPPort}}

# Database
DATABASE_DRIVER={{.DatabaseDriver}}
DATABASE_URL={{.DatabaseURL}}

# Security
ENCRYPTION_KEY={{.EncryptionKey}}
AUTH_SECRET={{.AuthSecret}}
AUTH_SERVICE={{.AuthService}}

# Cache
CACHE_DRIVER={{.CacheDriver}}

# File Storage
FILES_DRIVER={{.FilesDriver}}
FILES_LOCAL_BASE_PATH={{.FilesLocalBasePath}}

# Legacy
LEGACY_PATH={{.LegacyPath}}

# Global API
GLOBAL_API_URL={{.GlobalAPIURL}}
`
