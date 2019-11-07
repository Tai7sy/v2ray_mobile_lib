package libv2ray

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/2dust/AndroidLibV2rayLite/VPN"
	mobasset "golang.org/x/mobile/asset"

	v2core "v2ray.com/core"
	v2net "v2ray.com/core/common/net"
	v2filesystem "v2ray.com/core/common/platform/filesystem"
	v2stats "v2ray.com/core/features/stats"
	v2serial "v2ray.com/core/infra/conf/serial"
	_ "v2ray.com/core/main/distro/all"
	v2internet "v2ray.com/core/transport/internet"

	v2applog "v2ray.com/core/app/log"
	v2commlog "v2ray.com/core/common/log"
)

const (
	v2Assert    = "v2ray.location.asset"
	assetperfix = "/dev/libv2rayfs0/asset"
)

/*V2RayPoint V2Ray Point Server
This is territory of Go, so no getter and setters!
*/
type V2RayPoint struct {
	SupportSet   V2RayVPNServiceSupportsSet
	statsManager v2stats.Manager

	dialer    *VPN.ProtectedDialer
	v2rayOP   sync.Mutex
	closeChan chan struct{}

	Vpoint    *v2core.Instance
	IsRunning bool

	DomainName           string
	ConfigureFileContent string
	AsyncResolve         bool
}

/*V2RayVPNServiceSupportsSet To support Android VPN mode*/
type V2RayVPNServiceSupportsSet interface {
	Setup(Conf string) int
	Prepare() int
	Shutdown() int
	Protect(int) bool
	OnEmitStatus(int, string) int
}

/*RunLoop Run V2Ray main loop
 */
func (v *V2RayPoint) RunLoop(prefIPv6 bool) (err error) {
	v.v2rayOP.Lock()
	defer v.v2rayOP.Unlock()
	//Construct Context

	if !v.IsRunning {
		v.closeChan = make(chan struct{})
		v.dialer.PrepareResolveChan()
		go func() {
			select {
			// wait until resolved
			case <-v.dialer.ResolveChan():
				// shutdown VPNService if server name can not reolved
				if !v.dialer.IsVServerReady() {
					log.Println("vServer cannot resolved, shutdown")
					v.StopLoop()
					v.SupportSet.Shutdown()
				}

			// stop waiting if manually closed
			case <-v.closeChan:
			}
		}()

		if v.AsyncResolve {
			go v.dialer.PrepareDomain(v.DomainName, v.closeChan, prefIPv6)
		} else {
			v.dialer.PrepareDomain(v.DomainName, v.closeChan, prefIPv6)
		}

		err = v.pointloop()
	}
	return
}

/*StopLoop Stop V2Ray main loop
 */
func (v *V2RayPoint) StopLoop() (err error) {
	v.v2rayOP.Lock()
	defer v.v2rayOP.Unlock()
	if v.IsRunning {
		close(v.closeChan)
		v.shutdownInit()
		v.SupportSet.OnEmitStatus(0, "Closed")
	}
	return
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
	v.IsRunning = false
	v.Vpoint.Close()
	v.Vpoint = nil
	v.statsManager = nil
}

func (v *V2RayPoint) pointloop() error {
	log.Println("loading v2ray config")
	config, err := v2serial.LoadJSONConfig(strings.NewReader(v.ConfigureFileContent))
	if err != nil {
		log.Println(err)
		return err
	}

	log.Println("new v2ray core")
	v.Vpoint, err = v2core.New(config)
	if err != nil {
		v.Vpoint = nil
		log.Println(err)
		return err
	}
	v.statsManager = v.Vpoint.GetFeature(v2stats.ManagerType()).(v2stats.Manager)

	log.Println("start v2ray core")
	v.IsRunning = true
	if err := v.Vpoint.Start(); err != nil {
		v.IsRunning = false
		log.Println(err)
		return err
	}

	v.SupportSet.Prepare()
	v.SupportSet.Setup("")
	v.SupportSet.OnEmitStatus(0, "Running")
	return nil
}

func (v *V2RayPoint) MeasureDelay() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)

	go func() {
		select {
		case <-v.closeChan:
			// cancel request if close called during meansure
			cancel()
		case <-ctx.Done():
		}
	}()

	return measureInstDelay(ctx, v.Vpoint)
}

func initV2Env() {
	if os.Getenv(v2Assert) != "" {
		return
	}
	//Initialize asset API, Since Raymond Will not let notify the asset location inside Process,
	//We need to set location outside V2Ray
	os.Setenv(v2Assert, assetperfix)
	//Now we handle read
	v2filesystem.NewFileReader = func(path string) (io.ReadCloser, error) {
		if strings.HasPrefix(path, assetperfix) {
			p := path[len(assetperfix)+1:]
			//is it overridden?
			//by, ok := overridedAssets[p]
			//if ok {
			//	return os.Open(by)
			//}
			return mobasset.Open(p)
		}
		return os.Open(path)
	}
}

//Delegate Funcation
func TestConfig(ConfigureFileContent string) error {
	initV2Env()
	_, err := v2serial.LoadJSONConfig(strings.NewReader(ConfigureFileContent))
	return err
}

func MeasureOutboundDelay(ConfigureFileContent string) (int64, error) {
	initV2Env()
	config, err := v2serial.LoadJSONConfig(strings.NewReader(ConfigureFileContent))
	if err != nil {
		return -1, err
	}

	// dont listen to anything for test purpose
	config.Inbound = nil
	config.Transport = nil
	// keep only basic features
	config.App = config.App[:4]

	inst, err := v2core.New(config)
	if err != nil {
		return -1, err
	}

	inst.Start()
	delay, err := measureInstDelay(context.Background(), inst)
	inst.Close()
	return delay, err
}

/*NewV2RayPoint new V2RayPoint*/
func NewV2RayPoint(s V2RayVPNServiceSupportsSet, adns bool) *V2RayPoint {
	initV2Env()

	// inject our own log writer
	v2applog.RegisterHandlerCreator(v2applog.LogType_Console,
		func(lt v2applog.LogType,
			options v2applog.HandlerCreatorOptions) (v2commlog.Handler, error) {
			return v2commlog.NewLogger(createStdoutLogWriter()), nil
		})

	dialer := VPN.NewPreotectedDialer(s)
	v2internet.UseAlternativeSystemDialer(dialer)
	return &V2RayPoint{
		SupportSet:   s,
		dialer:       dialer,
		AsyncResolve: adns,
	}
}

func CheckVersion() int {
	return 21
}

/*CheckVersionX string
This func will return libv2ray binding version and V2Ray version used.
*/
func CheckVersionX() string {
	return fmt.Sprintf("Libv2rayLite V%d, Core V%s", CheckVersion(), v2core.Version())
}

func measureInstDelay(ctx context.Context, inst *v2core.Instance) (int64, error) {
	if inst == nil {
		return -1, errors.New("core instance nil")
	}

	tr := &http.Transport{
		TLSHandshakeTimeout: 6 * time.Second,
		DisableKeepAlives:   true,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dest, err := v2net.ParseDestination(fmt.Sprintf("%s:%s", network, addr))
			if err != nil {
				return nil, err
			}
			return v2core.Dial(ctx, inst, dest)
		},
	}

	c := &http.Client{
		Transport: tr,
		Timeout:   12 * time.Second,
	}

	req, _ := http.NewRequestWithContext(ctx, "GET", "http://www.google.com/gen_204", nil)
	start := time.Now()
	resp, err := c.Do(req)
	if err != nil {
		return -1, err
	}
	if resp.StatusCode != http.StatusNoContent {
		return -1, fmt.Errorf("status != 204: %s", resp.Status)
	}
	resp.Body.Close()
	return time.Since(start).Milliseconds(), nil
}
