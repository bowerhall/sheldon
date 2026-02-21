module github.com/bowerhall/sheldon

go 1.24.0

toolchain go1.24.5

require (
	github.com/anthropics/anthropic-sdk-go v0.2.0-beta.3
	github.com/bowerhall/sheldonmem v0.0.0
	github.com/bwmarrin/discordgo v0.29.0
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/google/uuid v1.6.0
	github.com/joho/godotenv v1.5.1
	github.com/minio/minio-go/v7 v7.0.98
	github.com/robfig/cron/v3 v3.0.1
)

replace github.com/bowerhall/sheldonmem => ../pkg/sheldonmem

require (
	github.com/asg017/sqlite-vec-go-bindings v0.0.1-alpha.37 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-ini/ini v1.67.0 // indirect
	github.com/gorilla/websocket v1.4.2 // indirect
	github.com/klauspost/compress v1.18.2 // indirect
	github.com/klauspost/cpuid/v2 v2.2.11 // indirect
	github.com/klauspost/crc32 v1.3.0 // indirect
	github.com/minio/crc64nvme v1.1.1 // indirect
	github.com/minio/md5-simd v1.1.2 // indirect
	github.com/ncruces/go-sqlite3 v0.17.2-0.20240711235451-21de85e849b7 // indirect
	github.com/ncruces/julianday v1.0.0 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/xid v1.6.0 // indirect
	github.com/tetratelabs/wazero v1.7.3 // indirect
	github.com/tidwall/gjson v1.14.4 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	github.com/tinylib/msgp v1.6.1 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
