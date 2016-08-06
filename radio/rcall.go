package radio // import "tinyap.org/ping/radio"

//go:generate stringer -type=Err

import (
	"fmt"
	"time"
)

type RcallError string

func (e RcallError) Error() string {
	return string(e)
}

const (
	Nop = 0 + iota

	Trx // [1]size [1]Trx [2]timeout
	Rrx // [1]size [1]Rrx [Npkt]pkt

	Ttx // [1]size [1]Ttx [2]preamblems [Npkt]pkt
	Rtx // [1]size [1]Rtx

	Ttxrx // [1]size [1]Ttxrx [2]timeout  [2]preamblems [Npkt]pkt
	Rtxrx // [1]size [1]Rtxrx [Npkt]pkt

	Tping // [1]size [1]Tping
	Rping // [1]size [1]Rping

	Tmax

	Rerr = 128 // [1]size [1]Rerr [1]err

	Treset = 0xff - 1 // A special Rcall to reset the radio
	Rreset
)

const Npkt = 78
const CALLMAX = 1 + 1 + 2 + 2 + 1 + 1 + 78

type Err uint8

const (
	ErrMissing Err = 1 + iota
	ErrBadcall
	ErrTimeout
)

// A low-level representation of a radio call.
type Call struct {
	Type uint8
	Flag uint8

	Err      Err
	Timeout  time.Duration
	Preamble time.Duration

	Filterbyte3 uint8

	Pkt [Npkt]byte
}

func (r *Call) Bytes() ([]byte, error) {
	b := pbit8(nil, 0)

	b = pbit8(b, r.Type)
	b = pbit8(b, r.Flag)

	switch r.Type {
	default:
		return nil, RcallError("invalid rcall type")

	case Rtx, Tping, Rping:
		break

	case Trx:
		b = pbit16(b, uint16(r.Timeout.Nanoseconds()/1e6))
		b = pbit8(b, r.Filterbyte3)

	case Ttxrx:
		b = pbit16(b, uint16(r.Timeout.Nanoseconds()/1e6))
		b = pbit8(b, r.Filterbyte3)
		fallthrough
	case Ttx:
		b = pbit16(b, uint16(r.Preamble.Nanoseconds()/1e6))
		fallthrough
	case Rrx, Rtxrx:
		b = append(b, r.Pkt[:]...)

	case Rerr:
		b = pbit8(b, uint8(r.Err))
	}

	pbit8(b[0:0], uint8(len(b)))

	return b, nil
}

func UnmarshalRcall(b []byte) (r *Call, err error) {
	defer func() {
		if recover() != nil {
			println("bad rcall at ", b)
			r = nil
			err = RcallError("malformed Fcall")
		}
	}()

	n, b := gbit8(b)
	if len(b) != int(n)-1 {
		return nil, RcallError("bad length")
	}

	r = &Call{}

	r.Type, b = gbit8(b)
	r.Flag, b = gbit8(b)

	gtimeout := func(b []byte) (time.Duration, []byte) {
		var u16 uint16
		u16, b = gbit16(b)
		return time.Duration(u16) * time.Millisecond, b
	}

	switch r.Type {
	default:
		return nil, RcallError("invalid Rcall type")

	case Tping, Rping, Rtx:
		break

	case Trx:
		r.Timeout, b = gtimeout(b)
		r.Filterbyte3, b = gbit8(b)

	case Ttxrx:
		r.Timeout, b = gtimeout(b)
		r.Filterbyte3, b = gbit8(b)
		fallthrough
	case Ttx:
		r.Preamble, b = gtimeout(b)
		fallthrough
	case Rrx, Rtxrx:
		copy(r.Pkt[:], b[:Npkt])
		b = b[Npkt:]

	case Rerr:
		var u8 uint8
		u8, b = gbit8(b)
		r.Err = Err(u8)
	}

	return r, nil
}

func (r *Call) String() string {
	switch r.Type {
	case Trx:
		return fmt.Sprintf("Trx timeout %s", r.Timeout)
	case Rrx:
		return fmt.Sprintf("Rrx pkt %x", r.Pkt)

	case Ttxrx:
		return fmt.Sprintf("Ttxrx timeout %s preamble %s pkt %x", r.Timeout, r.Preamble, r.Pkt)
	case Rtxrx:
		return fmt.Sprintf("Rtxrx pkt %x", r.Pkt)

	case Tping:
		return fmt.Sprintf("Tping flag %x", r.Flag)
	case Rping:
		return fmt.Sprintf("Rping flag %x", r.Flag)

	case Rerr:
		return fmt.Sprintf("Rerr err %s", r.Err)

	default:
		return fmt.Sprintf("Unknown type %d", r.Type)
	}
}
