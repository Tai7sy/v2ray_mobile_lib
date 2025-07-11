package VPN

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/sys/unix"
	v2net "v2ray.com/core/common/net"
	v2internet "v2ray.com/core/transport/internet"
)

type protectSet interface {
	Protect(int) int
}

type resolved struct {
	domain       string
	IPs          []net.IP
	Port         int
	ipIdx        uint8
	ipLock       sync.Mutex
	lastSwitched time.Time
}

// NextIP switch to another resolved result.
// there still be race-condition here if multiple err concurently occured
// may cause idx keep switching,
// but that's an outside error can hardly handled here
func (r *resolved) NextIP() {
	r.ipLock.Lock()
	defer r.ipLock.Unlock()

	if len(r.IPs) > 1 {

		// throttle, don't switch too quickly
		now := time.Now()
		if now.Sub(r.lastSwitched) < time.Second*5 {
			log.Println("switch too quickly")
			return
		}
		r.lastSwitched = now
		r.ipIdx++

	} else {
		return
	}

	if r.ipIdx >= uint8(len(r.IPs)) {
		r.ipIdx = 0
	}

	cur := r.currentIP()
	log.Printf("switched to next IP: %s", cur)
}

func (r *resolved) currentIP() net.IP {
	if len(r.IPs) > 0 {
		return r.IPs[r.ipIdx]
	}

	return nil
}

// NewProtectedDialer ...
func NewProtectedDialer(p protectSet) *ProtectedDialer {
	d := &ProtectedDialer{
		// prefer native lookup on Android
		resolver:   &net.Resolver{PreferGo: false},
		protectSet: p,
	}
	return d
}

// ProtectedDialer ...
type ProtectedDialer struct {
	currentServer string
	resolveChan   chan struct{}

	vServer  *resolved
	resolver *net.Resolver

	protectSet
}

func (d *ProtectedDialer) IsVServerReady() bool {
	return (d.vServer != nil)
}

func (d *ProtectedDialer) PrepareResolveChan() {
	d.resolveChan = make(chan struct{})
}

func (d *ProtectedDialer) ResolveChan() <-chan struct{} {
	return d.resolveChan
}

// simplicated version of golang: internetAddrList in src/net/ipsock.go
func (d *ProtectedDialer) lookupAddr(addr string) (*resolved, error) {

	var (
		err        error
		host, port string
		portnum    int
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if host, port, err = net.SplitHostPort(addr); err != nil {
		log.Printf("PrepareDomain SplitHostPort Err: %v", err)
		return nil, err
	}

	if portnum, err = d.resolver.LookupPort(ctx, "tcp", port); err != nil {
		log.Printf("PrepareDomain LookupPort Err: %v", err)
		return nil, err
	}

	addrs, err := d.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("domain %s Failed to resolve", addr)
	}

	IPs := make([]net.IP, len(addrs))
	for i, ia := range addrs {
		IPs[i] = ia.IP
	}

	rs := &resolved{
		domain: host,
		IPs:    IPs,
		Port:   portnum,
	}

	return rs, nil
}

// PrepareDomain caches direct v2ray server host
func (d *ProtectedDialer) PrepareDomain(domainName string, closeCh <-chan struct{}) {
	log.Printf("Preparing Domain: %s", domainName)
	d.currentServer = domainName

	defer close(d.resolveChan)
	maxRetry := 10
	for {
		if maxRetry == 0 {
			log.Println("PrepareDomain maxRetry reached. exiting.")
			return
		}

		resolved, err := d.lookupAddr(domainName)
		if err != nil {
			maxRetry--
			log.Printf("PrepareDomain err: %v\n", err)
			select {
			case <-closeCh:
				log.Printf("PrepareDomain exit due to v2ray closed")
				return
			case <-time.After(time.Second * 2):
			}
			continue
		}

		d.vServer = resolved
		log.Printf("Prepare Result:\n Domain: %s\n Port: %d\n IPs: %v\n",
			resolved.domain, resolved.Port, resolved.IPs)
		return
	}
}

func (d *ProtectedDialer) getFd(network v2net.Network, isIPv4 bool) (fd int, err error) {
	switch network {
	case v2net.Network_TCP:
		if isIPv4 {
			fd, err = unix.Socket(unix.AF_INET, unix.SOCK_STREAM, unix.IPPROTO_TCP)
		} else {
			fd, err = unix.Socket(unix.AF_INET6, unix.SOCK_STREAM, unix.IPPROTO_TCP)
		}
		if err != nil {
			log.Printf("Failed to create socket: %v", err)
		}
	case v2net.Network_UDP:
		if isIPv4 {
			fd, err = unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_UDP)
		} else {
			fd, err = unix.Socket(unix.AF_INET6, unix.SOCK_DGRAM, unix.IPPROTO_UDP)
		}
		if err != nil {
			log.Printf("Failed to create socket: %v", err)
		}
	default:
		err = fmt.Errorf("unknown network")
	}
	return
}

// Dial exported as the protected dial method
func (d *ProtectedDialer) Dial(ctx context.Context,
	src v2net.Address, dest v2net.Destination, sockopt *v2internet.SocketConfig) (net.Conn, error) {

	network := dest.Network.SystemString()
	Address := dest.NetAddr()

	// v2ray server address,
	// try to connect fixed IP if multiple IP parsed from domain,
	// and switch to next IP if error occurred
	if strings.Compare(Address, d.currentServer) == 0 {
		if d.vServer == nil {
			log.Println("Dial pending prepare  ...", Address)
			<-d.resolveChan

			// user may close connection during PrepareDomain,
			// fast return release resources.
			if d.vServer == nil {
				return nil, fmt.Errorf("fail to prepare domain %s", d.currentServer)
			}
		}
		
		curIP := d.vServer.currentIP()
		log.Printf("Using Prepared: %s : %d", curIP, d.vServer.Port)

		isIPv4 := curIP.To4() != nil
		fd, err := d.getFd(dest.Network, isIPv4)
		if err != nil {
			return nil, err
		}

		conn, err := d.fdConn(ctx, curIP, d.vServer.Port, fd)
		if err != nil {
			d.vServer.NextIP()
			log.Printf("fdConn err: %v, try next IP", err)
			return nil, err
		}
		return conn, nil
	}

	// warning: if v2ray-core use a proxy-chain, will alawys not prepared!
	// v2ray connecting to "domestic" servers, no caching results
	log.Printf("Not Using Prepared: %s,%s", network, Address)
	resolved, err := d.lookupAddr(Address)
	if err != nil {
		return nil, err
	}

	isIPv4 := resolved.IPs[0].To4() != nil
	fd, err := d.getFd(dest.Network, isIPv4)
	if err != nil {
		return nil, err
	}

	// use the first resolved address.
	// the result IP may vary, eg: IPv6 addrs comes first if client has ipv6 address
	return d.fdConn(ctx, resolved.IPs[0], resolved.Port, fd)
}

func (d *ProtectedDialer) fdConn(ctx context.Context, ip net.IP, port int, fd int) (net.Conn, error) {

	defer unix.Close(fd)

	// call android VPN service to "protect" the fd connecting straight out
	d.Protect(fd)

	var err error
	if ip.To4() != nil {
		sa := &unix.SockaddrInet4{
			Port: port,
		}
		copy(sa.Addr[:], ip.To4())
		err = unix.Connect(fd, sa)
	} else {
		sa := &unix.SockaddrInet6{
			Port: port,
		}
		copy(sa.Addr[:], ip)
		err = unix.Connect(fd, sa)
	}

	if err != nil {
		log.Printf("fdConn unix.Connect err, Close Fd: %d Err: %v", fd, err)
		return nil, err
	}

	file := os.NewFile(uintptr(fd), "Socket")
	if file == nil {
		// returned value will be nil if fd is not a valid file descriptor
		log.Printf("fdConn fd invalid: %d", fd)
		return nil, errors.New("fdConn fd invalid")
	}

	defer file.Close()
	//Closing conn does not affect file, and closing file does not affect conn.
	conn, err := net.FileConn(file)
	if err != nil {
		log.Printf("fdConn FileConn Close Fd: %d Err: %v", fd, err)
		return nil, err
	}

	log.Printf("fdConn connection successful to %s:%d", ip, port)
	return conn, nil
}