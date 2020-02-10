Platdep=tun2socksBinary.386 tun2socksBinary.amd64 tun2socksBinary.arm64 tun2socksBinary.arm

deps:
	go get -u github.com/jteeuwen/go-bindata/...

tun2socksBinary.%:
	go-bindata -nomemcopy -pkg tun2socksBinarys -o ./binary_$*.go -tags $* ArchIndep/ ArchDep/$*/

binarys:deps $(Platdep)
	@echo "Done"

clean:
	-rm -f binary*
