# v2ray_mobile_lib

This library is used in [V2RayNG-android](https://github.com/Tai7sy/V2RayNG) for support V2Ray.

This library is used in [V2RayNG-iOS](https://github.com/Tai7sy/V2RayNG-iOS) for support V2Ray.

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
export PATH=$PATH:~/go/bin
export PATH=/Users/XXXX/Library/Java/JavaVirtualMachines/corretto-1.8.0_442/Contents/Home/bin:$PATH

# Build an AAR
make build_android

# Build a Framework
make build_ios

```
