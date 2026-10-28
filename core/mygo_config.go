//go:build !android

package core

import (
	"errors"
	"fmt"
	"github.com/metacubex/mihomo/hub"
	"github.com/metacubex/mihomo/listener"
)

func init() {
	registerMethod(CoreMethod("validateMyGoConfig"), withMessage(func(path string) error { _, err := loadConfig(path); return err }))
	registerMethod(CoreMethod("applyMyGoConfig"), withMessage(applyMyGoConfig))
	registerMethod(CoreMethod("checkMyGoListeners"), func(*MethodCall) (any, error) { return "", checkMyGoListeners() })
}
func checkMyGoListeners() error {
	configMu.Lock()
	defer configMu.Unlock()
	if currentConfig == nil || !isRunning.Load() {
		return nil
	}
	general := currentConfig.General
	ports := listener.GetPorts()
	for _, port := range []struct {
		name         string
		want, actual int
	}{{"mixed", general.MixedPort, ports.MixedPort}, {"HTTP", general.Port, ports.Port}, {"SOCKS", general.SocksPort, ports.SocksPort}, {"redir", general.RedirPort, ports.RedirPort}, {"TProxy", general.TProxyPort, ports.TProxyPort}} {
		if port.want != 0 && port.actual != port.want {
			return fmt.Errorf("%s listener could not bind port %d; check the core logs", port.name, port.want)
		}
	}
	if general.Tun.Enable && !listener.GetTunConf().Enable {
		return errors.New("TUN could not start; check the core logs and network permissions")
	}
	return nil
}

// A failed edit must leave the applied configuration in place while the native form shows the error.
func applyMyGoConfig(params *SetupParams) error {
	configMu.Lock()
	defer configMu.Unlock()
	if !isInit.Load() {
		return errNotInitialized
	}
	cfg, err := loadSetupConfig(params)
	if err != nil {
		return err
	}
	setTestURL(params.TestURL)
	skipCertVerify.Store(params.SkipCertVerify)
	currentConfig = cfg
	hub.ApplyConfig(cfg)
	patchSelectGroup(params.SelectedMap)
	bumpRouteEpoch()
	updateListeners(cfg)
	reconcileGeoUpdater()
	scheduleLoadMemoryRelease()
	return nil
}
