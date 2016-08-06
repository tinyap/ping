package radio // import "tinyap.org/ping/radio"

// from: 9fans.net/go/plan9/bit.go

func gbit8(b []byte) (uint8, []byte) {
	return uint8(b[0]), b[1:]
}

func gbit16(b []byte) (uint16, []byte) {
	return uint16(b[0])<<8 | uint16(b[1]), b[2:]
}

func gstring(b []byte) (string, []byte) {
	n, b := gbit16(b)
	return string(b[0:n]), b[n:]
}

func pbit8(b []byte, x uint8) []byte {
	n := len(b)
	if n+1 > cap(b) {
		nb := make([]byte, n, 100+2*cap(b))
		copy(nb, b)
		b = nb
	}
	b = b[0 : n+1]
	b[n] = x
	return b
}

func pbit16(b []byte, x uint16) []byte {
	n := len(b)
	if n+2 > cap(b) {
		nb := make([]byte, n, 100+2*cap(b))
		copy(nb, b)
		b = nb
	}
	b = b[0 : n+2]
	b[n] = byte(x >> 8)
	b[n+1] = byte(x)
	return b
}

func pstring(b []byte, s string) []byte {
	if len(s) >= 1<<16 {
		panic("string too long")
	}
	b = pbit16(b, uint16(len(s)))
	b = append(b, []byte(s)...)
	return b
}
