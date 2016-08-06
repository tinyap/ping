package pump // import "tinyap.org/ping/pump"
import "time"

// from 9fans.net/go/plan9

func gbit8(b []byte) (uint8, []byte) {
	return uint8(b[0]), b[1:]
}

func gbit16(b []byte) (uint16, []byte) {
	return uint16(b[0]) | uint16(b[1])<<8, b[2:]
}

func gbit32(b []byte) (uint32, []byte) {
	return uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24, b[4:]
}

func gbit32be(b []byte) (uint32, []byte) {
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), b[4:]
}

func gstring(b []byte) (string, []byte) {
	n, b := gbit16(b)
	return string(b[0:n]), b[n:]
}

func gtime(b []byte) (time.Time, []byte) {
	yearmonth, b := gbit8(b)
	year := 2007 + int(yearmonth&0xf)
	month := time.Month(1 + yearmonth>>4)
	day, b := gbit8(b)
	hour, b := gbit8(b)
	min, b := gbit8(b)
	return time.Date(year, month, int(day), int(hour), int(min), 0, 0, time.Local), b
}

func gdur(b []byte) (time.Duration, []byte) {
	h, b := gbit8(b)
	m, b := gbit8(b)
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute, b
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
	b[n] = byte(x)
	b[n+1] = byte(x >> 8)
	return b
}

func pbit32(b []byte, x uint32) []byte {
	n := len(b)
	if n+4 > cap(b) {
		nb := make([]byte, n, 100+2*cap(b))
		copy(nb, b)
		b = nb
	}
	b = b[0 : n+4]
	b[n] = byte(x)
	b[n+1] = byte(x >> 8)
	b[n+2] = byte(x >> 16)
	b[n+3] = byte(x >> 24)
	return b
}

func pbit32be(b []byte, x uint32) []byte {
	n := len(b)
	if n+4 > cap(b) {
		nb := make([]byte, n, 100+2*cap(b))
		copy(nb, b)
		b = nb
	}
	b = b[0 : n+4]
	b[n] = byte(x >> 24)
	b[n+1] = byte(x >> 16)
	b[n+2] = byte(x >> 8)
	b[n+3] = byte(x)
	return b
}
