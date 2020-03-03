// +build !android
// +build ios

package tun2socksBinarys

import (
	"runtime/debug"
)

type Tun2SocksRun struct {
	Status interface{}
}

func (v *Tun2SocksRun) CheckAndExport() error {
	return nil
}

func (v *Tun2SocksRun) Run(sendFd func() int) error {
	// ios 无法创建子进程, 使用内置的tun2socks

	// ios 内存限制很严格, 加快gc速度
	debug.SetGCPercent(10)
	return nil
}

func (v *Tun2SocksRun) Close() {
}
