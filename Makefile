BUILD_DIR=build
ASSERTS_DIR=$(BUILD_DIR)/assets
IOS_ARTIFACT=$(BUILD_DIR)/libv2ray.framework
ANDROID_ARTIFACT=$(BUILD_DIR)/libv2ray.aar
LDFLAGS='-s -w'
IMPORT_PATH=github.com/2dust/AndroidLibV2rayLite


assets:
	mkdir -p $(BUILD_DIR)
	# mkdir -p data assets && bash scripts/gen_assets.sh download && cp -v data/*.dat assets/
	mkdir -p $(ASSERTS_DIR)
	cd $(ASSERTS_DIR); curl https://raw.githubusercontent.com/2dust/AndroidLibV2rayLite/master/data/geosite.dat > geosite.dat
	cd $(ASSERTS_DIR); curl https://raw.githubusercontent.com/2dust/AndroidLibV2rayLite/master/data/geoip.dat > geoip.dat

goDeps:
	go get -d ./...
	go get golang.org/x/mobile/cmd/gomobile
	gomobile init
	# go get -u github.com/golang/protobuf/protoc-gen-go

tun2socksBinary:
	cd tun2socksBinary; $(MAKE) shippedBinary

init_env: assets goDeps tun2socksBinary
	@echo DONE

install_android_sdk_ubuntu:
	cd ~ ;curl -L https://raw.githubusercontent.com/Tai7sy/AndroidLibV2rayLite/master/.scripts/ubuntu-cli-install-android-sdk.sh | sudo bash -
	ls ~
	ls ~/android-sdk-linux/

build_android:
	gomobile bind -a -ldflags $(LDFLAGS) -tags json -target=android -o $(ANDROID_ARTIFACT) $(IMPORT_PATH)

build_ios:
	gomobile bind -a -ldflags $(LDFLAGS) -tags json -target=ios -o $(IOS_ARTIFACT) $(IMPORT_PATH)

clean:
	rm -rf $(ASSERTS_DIR)
	rm -rf $(BUILD_DIR)