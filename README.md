# AndroidLibV2rayLite

This library is used in [V2RayNG](https://github.com/Tai7sy/V2RayNG) for support V2Ray.

## Setup

```bash
make init_env
```


## Build
```bash
export http_proxy=http://127.0.0.1:10809
export https_proxy=http://127.0.0.1:10809
export ANDROID_HOME=/path/to/Android/Sdk
export ANDROID_NDK_HOME=/path/to/Android/android-ndk-r20b

# Build an AAR
make build_android

# Build a Framework (not test)
make build_ios

```
