//go:build (darwin || linux) && !android

package core

import (
	"os"
	"strconv"
)

// The MyGo launcher elevates only the child; its data remains owned by the desktop user.
func init() {
	if os.Geteuid() != 0 {
		return
	}
	uid, e := strconv.Atoi(os.Getenv("FLCLASH_OWNER_UID"))
	if e != nil || uid <= 0 {
		return
	}
	gid, e := strconv.Atoi(os.Getenv("FLCLASH_OWNER_GID"))
	if e != nil || gid < 0 {
		return
	}
	realUid, realGid = uid, gid
}
