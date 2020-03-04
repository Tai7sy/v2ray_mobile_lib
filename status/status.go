package status

import (
	v2core "v2ray.com/core"
)

type Status struct {
	IsRunning   bool
	PackageName string

	Vpoint v2core.Server
}

func CheckVersion() int {
	return 20
}

func (v *Status) GetDataDir() string {
	return v.PackageName
}

func (v *Status) GetApp(name string) string {
	return v.PackageName + name
}

func (v *Status) GetTun2socksArgs(localDNS bool, enableIPv6 bool) (ret []string) {
	ret = []string{"--netif-ipaddr",
		"26.26.26.4",
		"--netif-netmask",
		"255.255.255.0",
		"--socks-server-addr",
		"127.0.0.1:10808",
		"--tunmtu",
		"1500",
		"--loglevel",
		"notice",
		"--enable-udprelay",
		"--sock-path",
		v.GetDataDir() + "sock_path", // android vpn 建立起来之后返回的fd, 用来获取数据
	}

	if enableIPv6 {
		ret = append(ret, "--netif-ip6addr", "da26:2626::4")
	}

	if localDNS {
		ret = append(ret, "--dnsgw", "127.0.0.1:10807")
	}

	return
}

func (v *Status) GetVPNSetupArg(localDNS bool, enableIPv6 bool) (ret string) {
	ret = "m,1500 a,26.26.26.1,24 r,0.0.0.0,0"

	if enableIPv6 {
		ret += " a,da26:2626::1,120 r,::,0" // 这里 da26:2626::1/120 = 255 ips
	}
	if localDNS {
		ret += " d,26.26.26.4"
	}
	return
}
