//go:build windows

package coreclient

import (
	"crypto/rand"
	"encoding/hex"
	"net"

	"github.com/Microsoft/go-winio"
	"golang.org/x/sys/windows"
)

func listen() (net.Listener, string, func(), error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return nil, "", nil, err
	}
	token := windows.GetCurrentProcessToken()
	user, err := token.GetTokenUser()
	if err != nil {
		return nil, "", nil, err
	}
	address := `\\.\pipe\FlClashMyGo_` + hex.EncodeToString(b)
	ln, err := winio.ListenPipe(address, &winio.PipeConfig{SecurityDescriptor: "D:P(A;;GA;;;" + user.User.Sid.String() + ")", InputBufferSize: 65536, OutputBufferSize: 65536})
	return ln, address, func() {}, err
}
