module github.com/Tai7sy/v2ray_mobile_lib

go 1.13

require (
	github.com/kevinburke/go-bindata v3.16.0+incompatible // indirect
	go.starlark.net v0.0.0-20200203144150-6677ee5c7211 // indirect
	golang.org/x/mobile v0.0.0-20200212152714-2b26a4705d24
	golang.org/x/sys v0.0.0-20191002063906-3421d5a6bb1c
	v2ray.com/core v4.19.1+incompatible
)

replace v2ray.com/core => ../v2ray.com/core
