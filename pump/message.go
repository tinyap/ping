package pump

import (
	"errors"
	"fmt"
	"time"
)

//go:generate stringer -type=BolusStatus

type Amount int64

const (
	Milliunit Amount = 1
	Unit             = 1000 * Milliunit
)

func (a Amount) Milliunits() int64 { return int64(a) }
func (a Amount) String() string {
	return fmt.Sprintf("%.3fU", float64(a)/1000)
}

func (a Amount) Truncate(t Amount) Amount {
	return Amount(int64(a) - int64(a)%int64(t))
}

type Rate int64

const (
	MilliunitsPerHour Rate = 1
	UnitsPerHour           = 1000 * MilliunitsPerHour
)

func (r Rate) MilliunitsPerHour() int64 { return int64(r) }

func (r Rate) Total(d time.Duration) Amount {
	return Amount(d.Minutes() / 60 * float64(r))
}

func (r Rate) String() string {
	return fmt.Sprintf("%.3fU/hr", float64(r)/1000)
}

type Arg interface {
	Marshal() []byte
}

type Reply interface {
	Unmarshal([]byte) error
}

type Empty struct{}

func (e *Empty) Marshal() []byte {
	return []byte{}
}
func (e *Empty) Unmarshal(b []byte) error {
	if len(b) != 0 {
		return errors.New("expected empty message")
	} else {
		return nil
	}
}

type Wakeup struct{}

func (w *Wakeup) Marshal() []byte {
	// This is a hardcoded value that seems to be dependent
	// on firmware version, or remote.
	return []byte{0x49, 0x01, 0x2d, 0x14}
}

type Keepalive struct {
	Backoff time.Duration
}

func (k *Keepalive) Unmarshal(b []byte) error {
	//	if len(b) != 2 {
	//		return errors.New("bad frame size")
	//	}

	ms, _ := gbit16(b)
	k.Backoff = time.Duration(ms) * time.Millisecond

	return nil
}

// The "home screen" status message of the pump.
// This seems exactly tailored to render the remote/pump
// home screen.
type Status struct {
	Warn bool // true when a warning is active

	Now time.Time // Current pump time

	Basal Rate // Current basal rate

	Reservoir Amount // Amount left in reservoir

	Temp          int8          // Current temp (%)
	TempRemaining time.Duration // Time remaining on temp
	TempDuration  time.Duration // Total duration of temp
}

func (s *Status) String() string {
	if int64(s.Temp) != 0 {
		return fmt.Sprintf("Status %s basal %s reservoir %s temp %d %s (%s)",
			s.Now.Format(time.Kitchen), s.Basal, s.Reservoir, s.Temp, s.TempRemaining, s.TempDuration)
	} else {
		return fmt.Sprintf("Status %s basal %s reservoir %s", s.Now.Format(time.Kitchen), s.Basal, s.Reservoir)
	}
}

func (s *Status) Unmarshal(b []byte) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New(fmt.Sprintf("bad packet at %x", b))
		}
	}()

	flag, b := gbit8(b)
	s.Warn = flag&0x10 == 0x10

	// 3 unknown
	b = b[3:]

	// Current time
	s.Now, b = gtime(b)

	// 4 unknown
	b = b[4:]

	basal, b := gbit16(b)
	s.Basal = Rate(basal) * MilliunitsPerHour
	reservoir, b := gbit8(b)
	s.Reservoir = Amount(reservoir) * Unit

	// 2 unknown
	b = b[2:]

	tempFlag, b := gbit8(b)
	if tempFlag&0x1 == 0x1 {
		var temp uint8
		temp, b = gbit8(b)
		if temp > 128 {
			s.Temp = -int8((256 - int(temp)))
		} else {
			s.Temp = int8(temp)
		}

		// 1 unknown
		b = b[1:]

		s.TempRemaining, b = gdur(b)
		s.TempDuration, b = gdur(b)
	}

	return nil

}

type Status2 struct {
	BolusTime  time.Time
	Bolus, IOB Amount
}

func (s *Status2) String() string {
	return fmt.Sprintf("Status2 bolus time %s bolus %s IOB %s",
		s.BolusTime.Format(time.Kitchen), s.Bolus, s.IOB)
}

func (s *Status2) Unmarshal(b []byte) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New(fmt.Sprintf("packet error at %x", b))
		}
	}()

	b = b[4:]
	u16, b := gbit16(b)
	s.Bolus = Amount(u16) * Milliunit
	s.BolusTime, b = gtime(b)

	b = b[6:]
	u16, b = gbit16(b)
	s.IOB = Amount(10*u16) * Milliunit

	return
}

type Status3 struct {
	DailyBolus, DailyBasal Amount
	Temp, Suspend          bool
}

func (s *Status3) String() string {
	return fmt.Sprintf("Status3 bolus %s basal %s temp %v suspend %v",
		s.DailyBolus, s.DailyBasal, s.Temp, s.Suspend)
}

func (s *Status3) Unmarshal(b []byte) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New(fmt.Sprintf("packet error at %x", b))
		}
	}()

	b = b[2:]
	u8, b := gbit8(b)
	s.Temp = u8&0x1 == 0x1
	s.Suspend = u8&0x2 == 0x2

	b = b[1:]
	u32, b := gbit32(b)
	s.DailyBolus = Amount(u32) * Milliunit
	u32, b = gbit32(b)
	s.DailyBasal = Amount(u32) * Milliunit

	return
}

type Status4 struct {
	Active     bool
	Start, End time.Time

	Delivered, Total Amount
}

func (s *Status4) String() string {
	return fmt.Sprintf("Status4 active %v time %s-%s delivered %s/%s",
		s.Active, s.Start.Format(time.Kitchen), s.End.Format(time.Kitchen),
		s.Delivered, s.Total)
}

func (s *Status4) Unmarshal(b []byte) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New(fmt.Sprintf("packet error at %x", b))

		}
	}()

	b = b[1:]

	u8, b := gbit8(b)
	s.Active = u8&0x1 == 0x1
	s.Start, b = gtime(b)

	hh, b := gbit8(b)
	mm, b := gbit8(b)

	mm1 := s.Start.Hour()*60 + s.Start.Minute()
	mm2 := int(hh)*60 + int(mm)
	diff := (mm2 - mm1) * 60
	if diff < 0 {
		diff += 24 * 60 * 60
	}
	s.End = s.Start.Add(time.Duration(diff) * time.Second)

	u16, b := gbit16(b)
	s.Delivered = Amount(u16) * Milliunit
	u16, b = gbit16(b)
	s.Total = Amount(u16) * Milliunit

	return
}

type Clearwarn struct{}

func (c *Clearwarn) String() string {
	return "Clearwarn"
}

func (c *Clearwarn) Marshal() []byte {
	return []byte{0xa7, 0x01}
}

type Bolus struct {
	Bolus    Amount
	Duration time.Duration
}

func (b *Bolus) String() string {
	return fmt.Sprintf("Bolus %s %s", b.Bolus, b.Duration)
}

func (b *Bolus) Marshal() []byte {
	if int(b.Duration.Minutes())%6 != 0 {
		// TODO: Marshal to return error
		panic("combo duration must be multiple of 6")
	}

	var combo uint8 = 0
	if b.Duration != 0 {
		combo = 0x01
	}

	buf := pbit8(nil, combo)
	buf = pbit8(buf, 0)

	buf = pbit16(buf, uint16(b.Bolus.Milliunits()))
	buf = pbit16(buf, 0xffff^uint16(b.Bolus.Milliunits()))

	buf = pbit8(buf, uint8(b.Duration.Minutes()/6))

	// Fill the rest with zeroes.
	n := 28 - len(buf)
	for i := 0; i < n; i++ {
		buf = pbit8(buf, 0)
	}
	return buf
}

func (b *Bolus) Unmarshal(buf []byte) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New(fmt.Sprintf("frame error at %x", b))
		}
	}()

	buf = buf[2:]

	u16, buf := gbit16(buf)
	b.Bolus = Amount(u16) * Milliunit
	u16, buf = gbit16(buf)
	b.Duration = time.Duration(u16*6) * time.Minute
	return
}

type BolusStatus uint8

const (
	BolusUnknown BolusStatus = 0 + iota
	BolusBusy
	BolusDone
)

type Deliverystatus struct {
	Status BolusStatus
}

func (d *Deliverystatus) String() string {
	return fmt.Sprintf("Deliverystatus %s", d.Status)
}

func (d *Deliverystatus) Unmarshal(b []byte) (err error) {
	defer func() {
		if recover() != nil {
			err = errors.New(fmt.Sprintf("frame error at %x", b))
		}
	}()

	// 1 unknown
	b = b[1:]

	flag, b := gbit8(b)
	switch flag {
	case 0x01:
		d.Status = BolusBusy
	case 0x02:
		d.Status = BolusDone
	default:
		d.Status = BolusUnknown
	}

	return
}