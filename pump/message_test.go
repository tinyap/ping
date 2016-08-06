package pump

import (
	"encoding/hex"
	"testing"
	"time"
)

func mustDecode(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic(err)
	}
	return b
}

var status_NoTemp = mustDecode("0103000059040e0300000000fa0008000000000000000400")

func TestStatus_NoTemp(t *testing.T) {
	s := new(Status)
	if err := s.Unmarshal(status_NoTemp); err != nil {
		t.Error(err)
	}

	if !time.Date(2016, 6, 4, 14, 3, 0, 0, time.Local).Equal(s.Now) {
		t.Error("bad time")
	}

	if s.Basal != 250*MilliunitsPerHour {
		t.Error("bad basal")
	}

	if s.Reservoir != 8*Unit {
		t.Error("bad reservoir")
	}

	if s.Temp != 0 {
		t.Error("bad temp")
	}
}

var status = mustDecode("0103000059050f0a00000000fa0008000001baff040c041e")

func TestStatus(t *testing.T) {
	s := new(Status)
	if err := s.Unmarshal(status); err != nil {
		t.Error(err)
	}

	if !time.Date(2016, 6, 5, 15, 10, 0, 0, time.Local).Equal(s.Now) {
		t.Error("bad time")
	}

	if s.Basal != 250*MilliunitsPerHour {
		t.Error("bad basal")
	}

	if s.Reservoir != 8*Unit {
		t.Error("bad reservoir")
	}

	if s.Temp != -70 {
		t.Error("bad temp")
	}

	if s.TempRemaining != 4*time.Hour+12*time.Minute {
		t.Error("bad temp remaining")
	}

	if s.TempDuration != 4*time.Hour+30*time.Minute {
		t.Error("bad temp duration")
	}
}

var status_temp = mustDecode("0103000059050f1700000000fa000800000128000018001e")

func TestStatus_temp(t *testing.T) {
	s := new(Status)
	if err := s.Unmarshal(status_temp); err != nil {
		t.Error(err)
	}

	if !time.Date(2016, 6, 5, 15, 23, 0, 0, time.Local).Equal(s.Now) {
		t.Error("bad time")
	}

	if s.Basal != 250*MilliunitsPerHour {
		t.Error("bad basal")
	}

	if s.Reservoir != 8*Unit {
		t.Error("bad reservoir")
	}

	if s.Temp != 40 {
		t.Error("bad temp")
	}

	if s.TempRemaining != 24*time.Minute {
		t.Error("bad temp remaining")
	}

	if s.TempDuration != 30*time.Minute {
		t.Error("bad temp duration")
	}
}

func TestStatus_bad(t *testing.T) {
	s := new(Status)
	err := s.Unmarshal([]byte{1, 2, 3})
	if err == nil {
		t.Error("expected error")
	}
}

var status2 = mustDecode("01290100fa0059051209a0860100b88818003b0400000700")

func TestStatus2(t *testing.T) {
	s := new(Status2)
	err := s.Unmarshal(status2)
	if err != nil {
		t.Error(err)
	}

	if !time.Date(2016, 6, 5, 18, 9, 0, 0, time.Local).Equal(s.BolusTime) {
		t.Error("bad time")
	}

	if s.Bolus != 250*Milliunit {
		t.Error("bad bolus")
	}

	if s.IOB != 240*Milliunit {
		t.Error("bad IOB")
	}
}

var status3 = mustDecode("0164e107fa00000099030000")

func TestStatus3(t *testing.T) {
	s := new(Status3)
	err := s.Unmarshal(status3)
	if err != nil {
		t.Error(err)
	}

	if !s.Temp {
		t.Error("bad temp flag")
	}

	if s.Suspend {
		t.Error("bad suspend flag")
	}

	if s.DailyBolus != 250*Milliunit {
		t.Error("bad daily bolus")
	}

	if s.DailyBasal != 921*Milliunit {
		t.Error("bad daily basal")
	}
}

var status4 = mustDecode("0101590516210221c800e80300000400")

func TestStatus4(t *testing.T) {
	s := new(Status4)
	if err := s.Unmarshal(status4); err != nil {
		t.Error(err)
	}

	if !s.Active {
		t.Error("bad active")
	}

	if !time.Date(2016, 6, 5, 22, 33, 0, 0, time.Local).Equal(s.Start) {
		t.Error("bad start")
	}
	if !time.Date(2016, 6, 6, 2, 33, 0, 0, time.Local).Equal(s.End) {
		t.Error("bad end")
	}

	if s.Delivered != 200*Milliunit {
		t.Error("bad delivered")
	}

	if s.Total != 1*Unit {
		t.Error("bad total")
	}
}

var status4_cancelled = mustDecode("0102590516211625d200e80300000000")

func TestStatus4_cancelled(t *testing.T) {
	s := new(Status4)
	if err := s.Unmarshal(status4_cancelled); err != nil {
		t.Error(err)
	}

	if s.Active {
		t.Error("bad active")
	}

	if !time.Date(2016, 6, 5, 22, 33, 0, 0, time.Local).Equal(s.Start) {
		t.Error("bad start")
	}
	if !time.Date(2016, 6, 5, 22, 37, 0, 0, time.Local).Equal(s.End) {
		t.Error("bad end")
	}

	if s.Delivered != 210*Milliunit {
		t.Error("bad delivered")
	}

	if s.Total != 1*Unit {
		t.Error("bad total")
	}
}
