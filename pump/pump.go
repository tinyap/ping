package pump

import (
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

type Stat struct {
	Now time.Time

	Basal     Rate
	Reservoir Amount

	IOB, LastBolus Amount

	Temp               int
	TempBegin, TempEnd time.Time

	ComboActive                bool
	ComboBegin, ComboEnd       time.Time
	ComboDelivered, ComboTotal Amount

	DailyBasal, DailyBolus Amount

	Warn bool
}

func (s *Stat) String() string {
	parts := []string{
		fmt.Sprintf("%s warn %v basal %s", s.Now.Format(time.Kitchen), s.Warn, s.Basal),
		fmt.Sprintf("reservoir %s IOB %s lastbolus %s", s.Reservoir, s.IOB, s.LastBolus),
		fmt.Sprintf("temp %d %s-%s", s.Temp,
			s.TempBegin.Format(time.Kitchen),
			s.TempEnd.Format(time.Kitchen)),
		fmt.Sprintf("combo %v %s-%s %s/%s",
			s.ComboActive, s.ComboBegin.Format(time.Kitchen),
			s.ComboEnd.Format(time.Kitchen), s.ComboDelivered, s.ComboTotal),
		fmt.Sprintf("basal %s bolus %s", s.DailyBasal, s.DailyBolus),
	}

	return strings.Join(parts, " ")
}

func (s *Stat) DailyInsulin() Amount {
	return s.DailyBasal + s.DailyBolus
}

// Carefully constructed to mimic what the remote would do, roughly.
func (p *Pump) Stat() (*Stat, error) {
	var s = new(Stat)

	if err := p.Resume(); err != nil {
		return nil, err
	}

	var status Status
	if err := p.Call(CallStatus, nil, &status); err != nil {
		return nil, err
	}
	s.Now = status.Now
	s.Reservoir = status.Reservoir
	s.Basal = status.Basal
	s.Temp = int(status.Temp)
	if s.Temp != 0 {
		s.TempBegin = s.Now.Add(status.TempRemaining).Add(-status.TempDuration)
		s.TempEnd = s.TempBegin.Add(status.TempDuration)
	}
	s.Warn = status.Warn

	var status4 Status4
	if err := p.Call(CallStatus4, nil, &status4); err != nil {
		return nil, err
	}
	s.ComboActive = status4.Active
	s.ComboBegin = status4.Start
	s.ComboEnd = status4.End
	s.ComboDelivered = status4.Delivered
	s.ComboTotal = status4.Total

	if err := p.Reset(); err != nil {
		return nil, err
	}

	var status2 Status2
	if err := p.Call(CallStatus2, nil, &status2); err != nil {
		return nil, err
	}
	s.LastBolus = status2.Bolus
	s.IOB = status2.IOB

	if err := p.Reset(); err != nil {
		return nil, err
	}

	// We discard results here; we're issuing this call only to get
	// the right sequence numbers.
	if err := p.Call(CallStatus, nil, nil); err != nil {
		return nil, err
	}

	var status3 Status3
	if err := p.Call(CallStatus3, nil, &status3); err != nil {
		return nil, err
	}
	s.DailyBasal = status3.DailyBasal
	s.DailyBolus = status3.DailyBolus

	p.Adjourn()

	return s, nil
}

func (p *Pump) CancelCombo() error {
	if err := p.Resume(); err != nil {
		return err
	}
	defer p.Adjourn()
	return p.Call(CallCancelcombo, nil, nil)
}

func (p *Pump) ClearWarn() error {
	if err := p.Resume(); err != nil {
		return err
	}

	defer p.Adjourn()
	return p.Call(CallCancelcombo, &Clearwarn{}, nil)
}

func (p *Pump) Bolus(bolus Amount, dur time.Duration) error {
	if int(dur.Minutes())%6 != 0 {
		return errors.New("combo duration must be increments of 6 minutes")
	}

	if err := p.Resume(); err != nil {
		return err
	}
	defer p.Adjourn()

	arg := &Bolus{Bolus: bolus, Duration: dur}
	var reply Bolus

	if err := p.Call(CallBolus, arg, &reply); err != nil {
		return err
	}

	if *arg != reply {
		return errors.New(fmt.Sprintf("pump returned mismatched bolus response: %s, expected %s"))
	}

	var callack uint8 = CallBolusack
	if reply.Duration != 0 {
		callack = CallComboack
	}

	if err := p.Call(callack, nil, nil); err != nil {
		return err
	}

Loop:
	for {
		var s Deliverystatus
		if err := p.Call(CallDeliverystatus, nil, &s); err != nil {
			return err
		}

		switch s.Status {
		case BolusBusy, BolusUnknown:
			if err := p.Call(CallDeliverycontinue, nil, nil); err != nil {
				return err
			}

		case BolusDone:
			break Loop
		}
	}

	return nil
}

// Convergent.
func (p *Pump) SetRate(log *log.Logger, rate Rate) (done bool, err error) {
	var stat *Stat
	if stat, err = p.Stat(); err != nil {
		return
	}

	if stat.Warn {
		if err = p.ClearWarn(); err != nil {
			return
		}
	}

	// TODO: rate should possibly take a separate nominator and denominator.

	// Compute marginal rate required to reach the desired rate.
	var need Rate
	scale := (100.0 + float64(stat.Temp)) / 100.0
	base := Rate(float64(stat.Basal) * scale)
	if base > rate {
		need = 0 * MilliunitsPerHour
	} else {
		need = rate - base
	}

	// Compute the combo that best matches our desired rate.
	var (
		min     Rate
		matched Rate
		total   Amount
		dur     time.Duration
	)
	durations := []time.Duration{
		30 * time.Minute,
		60 * time.Minute,
		90 * time.Minute,
		120 * time.Minute,
		180 * time.Minute,
		240 * time.Minute,
		300 * time.Minute,
	}
	for i, d := range durations {
		// The pump accepts 50 milliunit delivery increments.
		tot := need.Total(d).Truncate(50 * Milliunit)
		proposed := Rate((60/d.Minutes())*float64(tot)) * MilliunitsPerHour
		diff := rate - proposed
		if diff < 0 {
			diff = -diff
		}

		if i == 0 || diff < min {
			min = diff
			total = tot
			dur = d
			matched = proposed
		}
	}

	log.Printf("base %s need %s matched %s (%s/%s)", base, need, matched, total, dur)

	// Compute the current rate and finish if we're close.
	if stat.ComboActive {
		dur1 := stat.ComboEnd.Sub(stat.ComboBegin)
		total1 := stat.ComboTotal

		done = total == total1 && dur1 >= dur-5*time.Minute && dur1 <= dur+5*time.Minute
		if done {
			log.Printf("current combo (%s/%s) matches", total1, dur1)
			return
		}
	} else if total == 0*Milliunit {
		log.Printf("combo is off; total is zero")
		done = true
		return
	}

	// The existing combo isn't sufficient; we have to issue a new combo.
	if err = p.CancelCombo(); err != nil {
		return
	}

	log.Printf("setting new combo %s/%s", total, dur)
	err = p.Bolus(total, dur)
	return
}
