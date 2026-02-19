module github.com/kadet/kora

go 1.24.0

toolchain go1.24.5

require (
	github.com/anthropics/anthropic-sdk-go v0.2.0-beta.3
	github.com/go-telegram-bot-api/telegram-bot-api/v5 v5.5.1
	github.com/joho/godotenv v1.5.1
	github.com/kadet/koramem v0.0.0
)

replace github.com/kadet/koramem => ../pkg/koramem

require (
	github.com/asg017/sqlite-vec-go-bindings v0.0.1-alpha.37 // indirect
	github.com/ncruces/go-sqlite3 v0.17.2-0.20240711235451-21de85e849b7 // indirect
	github.com/ncruces/julianday v1.0.0 // indirect
	github.com/tetratelabs/wazero v1.7.3 // indirect
	github.com/tidwall/gjson v1.14.4 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	golang.org/x/sys v0.37.0 // indirect
)
