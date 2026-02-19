module mini-kv

go 1.21

require github.com/gorilla/websocket v1.5.3 // indirect

require mini-kv/server/console v0.0.0

replace mini-kv/server/console => ./console
