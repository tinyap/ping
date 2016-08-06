package pump // import "tinyap.org/ping/pump"

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"time"

	"tinyap.org/ping/radio"
)

var logframe = flag.Bool("logframe", false, "Logs frames as they are sent and received.")

const (
	CallWakeup           = 0x00
	CallKeepalive        = 0x03
	CallAdjourn          = 0x05
	CallStatus           = 0x50
	CallStatus1          = 0x20
	CallStatus2          = 0x25
	CallStatus3          = 0x27
	CallStatus4          = 0x26
	CallCancelcombo      = 0x35
	CallBolusack         = 0x30
	CallComboack         = 0x31
	CallDeliverycontinue = 0x32
	CallDeliverystatus   = 0x33
	CallBolus            = 0x37
	CallClearwarn        = 0x45
)

func typeString(typ uint8) string {
	switch typ {
	case CallWakeup:
		return "Wakeup"
	case CallKeepalive:
		return "Keepalive"
	case CallAdjourn:
		return "Adjourn"
	case CallStatus:
		return "Status"
	case CallStatus1:
		return "Status1"
	case CallStatus2:
		return "Status2"
	case CallStatus3:
		return "Status3"
	case CallStatus4:
		return "Status4"
	case CallCancelcombo:
		return "Cancelcombo"
	case CallBolusack:
		return "Bolusack"
	case CallComboack:
		return "Comboack"
	case CallDeliverycontinue:
		return "Deliverycontinue"
	case CallDeliverystatus:
		return "Deliverystatus"
	case CallBolus:
		return "Bolus"
	case CallClearwarn:
		return "Clearwarn"
	}

	return "<unknown>"
}

type frame struct {
	Type uint8
	Tag  uint8

	Body []byte
}

func (f *frame) Marshal() ([]byte, error) {
	b := pbit8(nil, f.Type)
	b = pbit8(b, 0)
	b = pbit8(b, f.Tag)
	b = pbit8(b, uint8(len(f.Body)))

	chk, ok := crc32hd(b)
	if !ok {
		return nil, errors.New(fmt.Sprintf("checksum missing for header %x", b))
	}

	b = pbit32(b, chk)

	if len(f.Body) > 0 {
		b = append(b, f.Body...)
		// This is an artifact of how these are stored and generated.
		b = pbit32be(b, crc32(f.Body))
	}

	return b, nil
}

func (f *frame) Unmarshal(b []byte) (err error) {
	hd := b[0:4]

	f.Type, b = gbit8(b)
	_, b = gbit8(b)
	f.Tag, b = gbit8(b)
	size, b := gbit8(b)

	chk, ok := crc32hd(hd)
	//	if !ok {
	//		return errors.New(fmt.Sprintf("checksum missing for header %x", hd))
	//	}

	chk1, b := gbit32(b)

	// TODO: make this an option, or at least be whiny about it.
	if ok && chk != chk1 {
		return errors.New(fmt.Sprintf("header checksum error: expected %x, got %x", chk, chk1))
	}

	if size == 0 {
		return nil
	}

	if len(b)-4 < int(size) {
		panic(1)
	}

	f.Body = b[0:size]
	b = b[size:]

	chk = crc32(f.Body)
	chk1, b = gbit32be(b)

	if chk != chk1 {
		return errors.New(fmt.Sprintf("payload checksum error: expected %x, got %x", chk, chk1))
	}

	return nil
}

func (f *frame) String() string {
	return fmt.Sprintf("type %s tag %02x body[%d] %x", typeString(f.Type), f.Tag, len(f.Body), f.Body)
}

type Pump struct {
	radio  *radio.Radio
	tagidx uint8
}

var tagSeq = []byte{
	0x00, 0x0e, 0xf8, 0x12, 0xea,
	0x24, 0xdc, 0x36, 0xc0, 0x4e,
	0xb6,
}

func New(radio *radio.Radio) *Pump {
	return &Pump{radio: radio}
}

func Dial(device, addr string) (*Pump, error) {
	r, err := radio.Dial(device, addr)
	if err != nil {
		return nil, err
	}
	return New(r), nil
}

func (p *Pump) nextTag() (uint8, error) {
	if int(p.tagidx) >= len(tagSeq) {
		return 0, errors.New("ran out of tags")
	}
	tag := tagSeq[p.tagidx]
	p.tagidx++
	return tag, nil
}

func (p *Pump) Adjourn() error {
	return p.Call(CallAdjourn, nil, nil)
}

func (p *Pump) Reset() error {
	p.Adjourn()
	return p.Resume()
}

func (p *Pump) Resume() error {
	// TODO: reset radio here too?

	p.tagidx = 0
	return p.Call(CallWakeup, &Wakeup{}, nil)
}

// Issue a high-level call to the pump, while taking care of
// session management and tag assignment. Call also sets
// preamble, timeout, and retry parameters that are appropriate
// for each call.
func (p *Pump) Call(typ uint8, arg Arg, reply Reply) error {
	var preamble, timeout time.Duration
	var tries int

	switch typ {
	case CallWakeup:
		preamble = 2 * time.Second
		timeout = 200 * time.Millisecond
		tries = 10

	default:
		timeout = 300 * time.Millisecond
		tries = 15
	}

	tx := &frame{Type: typ}
	if arg != nil {
		tx.Body = arg.Marshal()
	}

	if typ == CallAdjourn {
		return p.tx(tx)
	}

	rx := new(frame)
	if err := p.txrx(tx, rx, preamble, tries, timeout); err != nil {
		return err
	}

	// This is the pump's "i'm busy" message -- it asks us to
	// delay communication for a certain number of milliseconds.
	for rx.Type == CallKeepalive {
		var k Keepalive
		if err := k.Unmarshal(rx.Body); err != nil {
			return err
		}

		backoff := k.Backoff
		// This is a hack required to appease the pump -- it seems that
		// perhaps the remote that's shipped with the Ping isn't quite so
		// fast at waiting.
		if backoff == 300*time.Millisecond {
			backoff = 450 * time.Millisecond
		}
		time.Sleep(backoff)

		tx.Type = CallKeepalive
		tx.Body = nil

		if err := p.txrx(tx, rx, 0*time.Second, 10, 2*timeout); err != nil {
			return err
		}
	}

	if rx.Type != typ {
		return errors.New("unexpected reply type")
	}

	if reply != nil {
		return reply.Unmarshal(rx.Body)
	} else {
		return nil
	}
}

// Transmit and receive a frame with the given radio parameters.
// Preamble determines the amount time spent preambling the radio;
// tries specifies the total number of attempts to
// transmission/receipt; and timeout specifies how long to wait for a
// reply for each try. Note that only timeouts are retried.
func (p *Pump) txrx(tx *frame, rx *frame, preamble time.Duration, tries int, timeout time.Duration) error {
	var err error

	if tx.Tag, err = p.nextTag(); err != nil {
		return err
	}

	call := &radio.Call{
		Type:     radio.Ttxrx,
		Flag:     0,
		Timeout:  timeout,
		Preamble: preamble,
	}

	if *logframe {
		log.Printf("tx %s", tx)
	}

	pkt, err := tx.Marshal()
	if err != nil {
		return err
	}

	copy(call.Pkt[:], pkt)

call:
	reply, err := p.radio.Call(call)
	if err != nil {
		return err
	}

	if reply.Type == radio.Rerr {
		if reply.Err == radio.ErrTimeout && tries > 0 {
			tries--
			goto call
		}

		return fmt.Errorf("radio error %s", reply.Err)
	}

	if err := rx.Unmarshal(reply.Pkt[:]); err != nil {
		return err
	}

	if rx.Tag != tx.Tag^0xff {
		return errors.New("bad reply tag")
	}

	if *logframe {
		log.Printf("rx %s", rx)
	}

	return nil
}

func (p *Pump) tx(f *frame) error {
	var err error

	if f.Tag, err = p.nextTag(); err != nil {
		return err
	}

	call := &radio.Call{Type: radio.Ttx}

	if *logframe {
		log.Printf("tx! %s", f)
	}

	pkt, err := f.Marshal()
	if err != nil {
		return err
	}

	copy(call.Pkt[:], pkt)

	reply, err := p.radio.Call(call)
	if err != nil {
		return err
	}

	if reply.Type == radio.Rerr {
		return errors.New(fmt.Sprintf("radio error %s", reply.Err))
	}

	return nil
}