BUILD_DIR=build
IOS_ARTIFACT=$(BUILD_DIR)/libv2ray.framework
ANDROID_ARTIFACT=$(BUILD_DIR)/libv2ray.aar
LDFLAGS="-s -w"
IMPORT_PATH=github.com/Tai7sy/v2ray_mobile_lib

goDeps:
	go get -d ./...
	go get golang.org/x/mobile/cmd/gomobile
	gomobile init
	# go get -u github.com/golang/protobuf/protoc-gen-go

tun2socksBinarys:
	cd tun2socksBinarys; $(MAKE) binarys

init_env: clean goDeps tun2socksBinarys
	@echo DONE

install_android_sdk_ubuntu:
	cd ~ ;curl -L https://raw.githubusercontent.com/Tai7sy/AndroidLibV2rayLite/master/.scripts/ubuntu-cli-install-android-sdk.sh | sudo bash -
	ls ~
	ls ~/android-sdk-linux/

build_android:
	mkdir -p $(BUILD_DIR)
	gomobile bind -a -ldflags $(LDFLAGS) -tags json -target=android -o $(ANDROID_ARTIFACT) $(IMPORT_PATH)

build_ios:
	mkdir -p $(BUILD_DIR)
	gomobile bind -a -ldflags $(LDFLAGS) -tags json -target=ios -o $(IOS_ARTIFACT) $(IMPORT_PATH)

clean:
	rm -rf $(BUILD_DIR)