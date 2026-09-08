package api

import "routerprobe/internal/protocol"

func validObject(b []byte) error { return protocol.ValidObject(b) }
