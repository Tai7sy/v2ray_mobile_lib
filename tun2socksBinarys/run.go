// +build !ios

package tun2socksBinarys

import (
	"github.com/Tai7sy/v2ray_mobile_lib/process"
	"log"
	"os"
	"strconv"

	"github.com/Tai7sy/v2ray_mobile_lib/status"
)

type Tun2SocksRun struct {
	Status   *status.Status
	escorter *process.Escorting
}

func (v *Tun2SocksRun) checkIfRcExist() error {
	datadir := v.Status.GetDataDir()
	if _, err := os.Stat(datadir + strconv.Itoa(status.CheckVersion())); !os.IsNotExist(err) {
		log.Println("file exists")
		return nil
	}

	IndepDir, err := AssetDir("ArchIndep")
	log.Println(IndepDir)
	if err != nil {
		return err
	}
	for _, fn := range IndepDir {
		log.Println(datadir + "ArchIndep/" + fn)

		err := RestoreAsset(datadir, "ArchIndep/"+fn)
		log.Println(err)

		//GrantPremission
		os.Chmod(datadir+"ArchIndep/"+fn, 0700)
		log.Println(os.Remove(datadir + fn))
		log.Println(os.Symlink(datadir+"ArchIndep/"+fn, datadir+fn))
	}

	DepDir, err := AssetDir("ArchDep")
	log.Println(DepDir)
	if err != nil {
		return err
	}
	for _, fn := range DepDir {
		DepDir2, err := AssetDir("ArchDep/" + fn)
		log.Println("ArchDep/" + fn)
		if err != nil {
			return err
		}
		for _, FND := range DepDir2 {
			log.Println(datadir + "ArchDep/" + fn + "/" + FND)

			RestoreAsset(datadir, "ArchDep/"+fn+"/"+FND)
			os.Chmod(datadir+"ArchDep/"+fn+"/"+FND, 0700)
			log.Println(os.Remove(datadir + FND))
			log.Println(os.Symlink(datadir+"ArchDep/"+fn+"/"+FND, datadir+FND))
		}
	}
	s, _ := os.Create(datadir + strconv.Itoa(status.CheckVersion()))
	s.Close()

	return nil
}

func (v *Tun2SocksRun) CheckAndExport() error {
	return v.checkIfRcExist()
}

func (v *Tun2SocksRun) Run(sendFd func() int) error {
	if err := v.CheckAndExport(); err != nil {
		log.Println(err)
		return err
	}
	v.escorter = &process.Escorting{Status: v.Status}
	v.escorter.EscortingUp()

	go v.escorter.EscortRun(
		v.Status.GetApp("tun2socks"),
		v.Status.GetTun2socksArgs(v.EnableLocalDNS, v.ForwardIpv6), "",
		SendFd)
}

func (v *Tun2SocksRun) Close() {
	v.escorter.EscortingDown()
}
