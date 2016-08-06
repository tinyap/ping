package radio

import (
	"flag"
	"io"
	"log"

	"github.com/pkg/term"
)

var logradio = flag.Bool("logradio", false, "Log low level radio calls")

type RadioError string

func (err RadioError) Error() string {
	return string(err)
}

type Radio struct {
	rw io.ReadWriter
	// Reset() err
}

var openers = map[string]func(string) (io.ReadWriter, error){
	"tty": openSerial,
	"usb": openUSB,
}

func Dial(device, addr string) (*Radio, error) {
	open, ok := openers[device]
	if !ok {
		return nil, RadioError("invalid spec")
	}

	rw, err := open(addr)
	if err != nil {
		return nil, err
	}

	return &Radio{rw: rw}, nil
}

func New(rw io.ReadWriter) *Radio {
	return &Radio{rw: rw}
}

func (r *Radio) Call(req *Call) (*Call, error) {
	if *logradio {
		log.Printf("radio tx: %s", req)
	}

	bytes, err := req.Bytes()
	if err != nil {
		return nil, err
	}

	bytes, err = r.roundtrip(bytes)
	if err != nil {
		return nil, err
	}

	rep, err := UnmarshalRcall(bytes)
	if err != nil {
		return nil, err
	}

	if rep.Type != req.Type+1 && rep.Type != Rerr {
		return nil, RcallError("bad reply type")
	}

	if *logradio {
		log.Printf("radio rx: %s", rep)
	}

	return rep, nil
}

func (r *Radio) roundtrip(req []byte) ([]byte, error) {
	if _, err := r.rw.Write(req); err != nil {
		return nil, err
	}

	buf := make([]byte, CALLMAX)
	_, err := io.ReadFull(r.rw, buf[0:1])
	if err != nil {
		return nil, err
	}

	n, _ := gbit8(buf)
	if n > CALLMAX {
		return nil, RcallError("invalid Rcall length")
	}

	buf = buf[0:n]

	_, err = io.ReadFull(r.rw, buf[1:])
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func openSerial(dev string) (io.ReadWriter, error) {
	return term.Open(dev, term.Speed(19200), term.RawMode)
}

func openUSB(dev string) (io.ReadWriter, error) {
	rw, err := openSerial(dev)
	if err != nil {
		return nil, err
	}

	return &hexReadWriter{rw}, nil
}
