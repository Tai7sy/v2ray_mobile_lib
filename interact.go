package libv2ray

import (
	"fmt"
	"github.com/Tai7sy/v2ray_mobile_lib/VPN"
	"github.com/Tai7sy/v2ray_mobile_lib/status"
	"github.com/Tai7sy/v2ray_mobile_lib/tun2socksBinarys"
	_ "github.com/Tai7sy/v2ray_mobile_lib/v2ray"
	assets "golang.org/x/mobile/asset"
	"io"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	v2core "v2ray.com/core"
	v2filesystem "v2ray.com/core/common/platform/filesystem"
	v2stats "v2ray.com/core/features/stats"
	v2serial "v2ray.com/core/infra/conf/serial"
	v2internet "v2ray.com/core/transport/internet"

	v2applog "v2ray.com/core/app/log"
	v2commlog "v2ray.com/core/common/log"
)

const (
	v2Assert     = "v2ray.location.asset"
	assetsPrefix = "/dev/libv2rayfs0/assets"
)

/*V2RayPoint V2Ray Point Server
This is territory of Go, so no getter and setters!
*/
type V2RayPoint struct {
	SupportSet   V2RayVPNServiceSupportsSet
	statsManager v2stats.Manager

	dialer    *VPN.ProtectedDialer
	status    *status.Status
	tun2socks *tun2socksBinarys.Tun2SocksRun

	v2rayOP   *sync.Mutex
	closeChan chan struct{}

	PackageName          string
	DomainName           string
	ConfigureFileContent string
	EnableLocalDNS       bool
	ForwardIpv6          bool
}

/*V2RayVPNServiceSupportsSet To support Android VPN mode*/
type V2RayVPNServiceSupportsSet interface {
	Setup(Conf string) int
	Prepare() int
	Shutdown() int
	Protect(int) int
	OnEmitStatus(int, string) int
	SendFd() int
}

/*
RunLoop Run V2Ray main loop
*/
func (v *V2RayPoint) RunLoop() (err error) {
	v.v2rayOP.Lock()
	defer v.v2rayOP.Unlock()
	//Construct Context
	v.status.PackageName = v.PackageName

	if !v.status.IsRunning {

		// use protected dialer, prepare resolver
		if v.dialer != nil {
			v.closeChan = make(chan struct{})
			v.dialer.PrepareResolveChan()
			go v.dialer.PrepareDomain(v.DomainName, v.closeChan)
			go func() {
				select {
				// wait until resolved
				case <-v.dialer.ResolveChan():
					// shutdown VPNService if server name can not reolved
					if !v.dialer.IsVServerReady() {
						log.Println("vServer cannot resolved, shutdown")
						_ = v.StopLoop()
						v.SupportSet.Shutdown()
					}

				// stop waiting if manually closed
				case <-v.closeChan:
				}
			}()
		}

		err = v.pointLoop()
	}
	return
}

/*StopLoop Stop V2Ray main loop
 */
func (v *V2RayPoint) StopLoop() (err error) {
	v.v2rayOP.Lock()
	defer v.v2rayOP.Unlock()
	if v.status.IsRunning {
		if v.closeChan != nil {
			close(v.closeChan)
		}
		v.shutdownInit()
		v.SupportSet.OnEmitStatus(0, "Closed")
	}
	return
}

//Delegate Funcation
func (v *V2RayPoint) GetIsRunning() bool {
	return v.status.IsRunning
}

//Delegate Funcation
func (v V2RayPoint) QueryStats(tag string, direct string) int64 {
	if v.statsManager == nil {
		return 0
	}
	counter := v.statsManager.GetCounter(fmt.Sprintf("inbound>>>%s>>>traffic>>>%s", tag, direct))
	if counter == nil {
		return 0
	}
	return counter.Set(0)
}

func (v *V2RayPoint) shutdownInit() {
	v.status.IsRunning = false
	_ = v.status.Vpoint.Close()
	v.status.Vpoint = nil
	v.statsManager = nil
	v.tun2socks.Close()
}

func (v *V2RayPoint) pointLoop() error {
	if err := v.tun2socks.Run(v.SupportSet.SendFd); err != nil {
		log.Println(err)
		return err
	}

	log.Printf("EnableLocalDNS: %v\nForwardIpv6: %v\nDomainName: %s",
		v.EnableLocalDNS,
		v.ForwardIpv6,
		v.DomainName)

	runtime.GC()
	log.Println("loading v2ray config")
	config, err := v2serial.LoadJSONConfig(strings.NewReader(v.ConfigureFileContent))
	if err != nil {
		log.Println(err)
		return err
	}

	runtime.GC()
	log.Println("new v2ray core")
	inst, err := v2core.New(config)
	if err != nil {
		log.Println(err)
		return err
	}
	v.status.Vpoint = inst
	v.statsManager = inst.GetFeature(v2stats.ManagerType()).(v2stats.Manager)

	log.Println("start v2ray core")
	v.status.IsRunning = true
	if err := v.status.Vpoint.Start(); err != nil {
		v.status.IsRunning = false
		log.Println(err)
		return err
	}

	v.SupportSet.Prepare()
	v.SupportSet.Setup(v.status.GetVPNSetupArg(v.EnableLocalDNS, v.ForwardIpv6))
	v.SupportSet.OnEmitStatus(0, "Running")

	if v.PackageName == "ios" {
		// ios 内存限制很严格, 加快gc速度
		debug.SetGCPercent(10)
	}
	return nil
}

func initV2Env(assetsDirectory string) {
	if os.Getenv(v2Assert) != "" {
		return
	}
	if assetsDirectory != "" {
		// ios version, we pass assets path directly
		_ = os.Setenv(v2Assert, assetsDirectory)
	} else {
		// android
		//Initialize asset API, Since Raymond Will not let notify the asset location inside process,
		//We need to set location outside V2Ray
		_ = os.Setenv(v2Assert, assetsPrefix)
		//Now we handle the read
		v2filesystem.NewFileReader = func(path string) (io.ReadCloser, error) {
			if strings.HasPrefix(path, assetsPrefix) {
				p := path[len(assetsPrefix)+1:]
				//is it overridden?
				//by, ok := overridedAssets[p]
				//if ok {
				//	return os.Open(by)
				//}
				// https://stackoverflow.com/questions/49412762/gomobile-how-to-embed-assets-in-apk
				return assets.Open(p)
			}
			return os.Open(path)
		}
	}
}

//Delegate Function
func TestConfig(ConfigureFileContent string) error {
	initV2Env("")
	_, err := v2serial.LoadJSONConfig(strings.NewReader(ConfigureFileContent))
	return err
}

/*NewV2RayPoint new V2RayPoint*/
func NewV2RayPoint(s V2RayVPNServiceSupportsSet, assetsDirectory string, protectedDialer bool) *V2RayPoint {

	initV2Env(assetsDirectory)

	// inject our own log writer
	_ = v2applog.RegisterHandlerCreator(v2applog.LogType_Console,
		func(lt v2applog.LogType, options v2applog.HandlerCreatorOptions) (v2commlog.Handler, error) {
			return v2commlog.NewLogger(createStdoutLogWriter()), nil
		})

	selfStatus := &status.Status{}
	point := &V2RayPoint{
		SupportSet: s,
		v2rayOP:    new(sync.Mutex),
		status:     selfStatus,
		tun2socks:  &tun2socksBinarys.Tun2SocksRun{Status: selfStatus},
	}
	// use protected dialer to connect server directly without vpn
	if protectedDialer {
		point.dialer = VPN.NewProtectedDialer(s)
		v2internet.UseAlternativeSystemDialer(point.dialer)
	}
	return point
}

/*
CheckVersion int
This func will return libv2ray binding version.
*/
func CheckVersion() int {
	return status.CheckVersion()
}

/*
CheckVersionX string
This func will return libv2ray binding version and V2Ray version used.
*/
func CheckVersionX() string {
	return fmt.Sprintf("Libv2rayLite V%d, Core V%s", CheckVersion(), v2core.Version())
}
