package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"tinyap.org/ping/pump"

	"github.com/google/subcommands"
	"golang.org/x/net/context"
)

var radioFlag = flag.String("radio", "usb:/dev/cu.usbmodem000001", "The radio with which to talk to the pump.")

type printCmd struct {
	capitalize bool
}

func (*printCmd) Name() string     { return "print" }
func (*printCmd) Synopsis() string { return "Print args to stdout." }
func (*printCmd) Usage() string {
	return `print [-capitalize] <some text>:
  Print args to stdout.
`
}

type statCmd struct{ pump *pump.Pump }

func (*statCmd) Name() string             { return "stat" }
func (*statCmd) Synopsis() string         { return "Queries and prints pump statistics." }
func (*statCmd) Usage() string            { return "stat\n" }
func (*statCmd) SetFlags(f *flag.FlagSet) {}

func (s *statCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	stat, err := s.pump.Stat()
	if err != nil {
		log.Print(err)
		return subcommands.ExitFailure
	}

	tw := new(tabwriter.Writer)
	tw.Init(os.Stdout, 0, 4, 2, ' ', 0)

	fmt.Fprintf(tw, "time\t%s\n", stat.Now.Format(time.Stamp))
	fmt.Fprintf(tw, "reservoir\t%s\n", stat.Reservoir)
	fmt.Fprintf(tw, "basal\t%s\n", stat.Basal)
	fmt.Fprintf(tw, "last bolus\t%s\n", stat.LastBolus)
	fmt.Fprintf(tw, "IOB\t%s\n", stat.IOB)
	fmt.Fprintf(tw, "daily basal\t%s\n", stat.DailyBasal)
	fmt.Fprintf(tw, "daily bolus\t%s\n", stat.DailyBolus)

	if stat.Temp != 0 {
		fmt.Fprintf(tw, "temp\t%d %s-%s\n",
			stat.Temp, stat.TempBegin.Format(time.Kitchen),
			stat.TempEnd.Format(time.Kitchen))
	}
	if stat.ComboActive {
		fmt.Fprintf(tw, "combo\t%s/%s %s-%s\n",
			stat.ComboDelivered, stat.ComboTotal,
			stat.ComboBegin.Format(time.Kitchen),
			stat.ComboEnd.Format(time.Kitchen))
	}
	if stat.Warn {
		fmt.Fprintf(tw, "WARNING ACTIVE\n")
	}

	tw.Flush()

	return subcommands.ExitSuccess
}

type cancelComboCmd struct{ pump *pump.Pump }

func (*cancelComboCmd) Name() string             { return "cancelcombo" }
func (*cancelComboCmd) Synopsis() string         { return "Cancel the current combo" }
func (*cancelComboCmd) Usage() string            { return "cancelcombo\n" }
func (*cancelComboCmd) SetFlags(f *flag.FlagSet) {}

func (c *cancelComboCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if err := c.pump.CancelCombo(); err != nil {
		log.Print(err)
		return subcommands.ExitFailure
	}
	return subcommands.ExitSuccess
}

type setRateCmd struct{ pump *pump.Pump }

func (*setRateCmd) Name() string             { return "setrate" }
func (*setRateCmd) Synopsis() string         { return "Set a basal rate of insulin delivery" }
func (*setRateCmd) Usage() string            { return "setrate <rate>\n" }
func (*setRateCmd) SetFlags(f *flag.FlagSet) {}

func (s *setRateCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	if len(f.Args()) != 1 {
		return subcommands.ExitUsageError
	}

	in, err := strconv.ParseFloat(f.Args()[0], 64)
	if err != nil {
		log.Println(err)
		return subcommands.ExitUsageError
	}

	rate := pump.Rate(in*1000) * pump.MilliunitsPerHour

	l := log.New(os.Stderr, "setrate: ", 0)
	var done bool

	for !done {
		done, err = s.pump.SetRate(l, rate)
		if err != nil {
			log.Print(err)
			return subcommands.ExitFailure
		}
	}

	return subcommands.ExitSuccess
}

func main() {
	log.SetPrefix("")
	log.SetFlags(0)

	parts := strings.SplitN(*radioFlag, ":", 2)
	if len(parts) != 2 {
		flag.Usage()
		os.Exit(1)
	}

	var err error
	p, err := pump.Dial(parts[0], parts[1])
	if err != nil {
		log.Fatal(err)
	}

	subcommands.ImportantFlag("radio")
	subcommands.Register(subcommands.HelpCommand(), "")
	subcommands.Register(subcommands.FlagsCommand(), "")
	subcommands.Register(&statCmd{p}, "")
	subcommands.Register(&cancelComboCmd{p}, "")
	subcommands.Register(&setRateCmd{p}, "")

	flag.Parse()
	ctx := context.Background()
	os.Exit(int(subcommands.Execute(ctx)))
}
