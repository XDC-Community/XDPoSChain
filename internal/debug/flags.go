// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package debug

import (
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"

	"github.com/XinFinOrg/XDPoSChain/log"
	"github.com/XinFinOrg/XDPoSChain/log/term"
	"github.com/XinFinOrg/XDPoSChain/metrics"
	"github.com/XinFinOrg/XDPoSChain/metrics/exp"
	colorable "github.com/mattn/go-colorable"
	"gopkg.in/urfave/cli.v1"
)

var (
	VerbosityFlag = cli.IntFlag{
		Name:  "verbosity",
		Usage: "Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug, 5=detail",
		Value: 3,
	}
	vmoduleFlag = cli.StringFlag{
		Name:  "vmodule",
		Usage: "Per-module verbosity: comma-separated list of <pattern>=<level> (e.g. eth/*=5,p2p=4)",
		Value: "",
	}
	backtraceAtFlag = cli.StringFlag{
		Name:  "backtrace",
		Usage: "Request a stack trace at a specific logging statement (e.g. \"block.go:271\")",
		Value: "",
	}
	debugFlag = cli.BoolFlag{
		Name:  "debug",
		Usage: "Prepends log messages with call-site location (file and line number)",
	}
	pprofFlag = cli.BoolFlag{
		Name:  "pprof",
		Usage: "Enable the pprof HTTP server",
	}
	pprofPortFlag = cli.IntFlag{
		Name:  "pprofport",
		Usage: "pprof HTTP server listening port",
		Value: 6060,
	}
	pprofAddrFlag = cli.StringFlag{
		Name:  "pprofaddr",
		Usage: "pprof HTTP server listening interface",
		Value: "127.0.0.1",
	}
	memprofilerateFlag = cli.IntFlag{
		Name:  "memprofilerate",
		Usage: "Turn on memory profiling with the given rate",
		Value: runtime.MemProfileRate,
	}
	blockprofilerateFlag = cli.IntFlag{
		Name:  "blockprofilerate",
		Usage: "Turn on block profiling with the given rate",
	}
	cpuprofileFlag = cli.StringFlag{
		Name:  "cpuprofile",
		Usage: "Write CPU profile to the given file",
	}
	traceFlag = cli.StringFlag{
		Name:  "trace",
		Usage: "Write execution trace to the given file",
	}
	periodicProfilingFlag = cli.BoolFlag{
		Name:  "periodicprofile",
		Usage: "Periodically profile cpu and memory status",
	}
	debugDataDirFlag = cli.StringFlag{
		Name:  "debugdatadir",
		Usage: "Debug Data directory for profiling output",
	}
)

// Flags holds all command-line flags required for debugging.
var Flags = []cli.Flag{
	VerbosityFlag,
	//vmoduleFlag,
	//backtraceAtFlag,
	debugFlag,
	pprofFlag,
	pprofAddrFlag,
	pprofPortFlag,
	memprofilerateFlag,
	//blockprofilerateFlag,
	cpuprofileFlag,
	//traceFlag,
	periodicProfilingFlag,
	debugDataDirFlag,
}

var Glogger *log.GlogHandler

func init() {
	usecolor := term.IsTty(os.Stderr.Fd()) && os.Getenv("TERM") != "dumb"
	output := io.Writer(os.Stderr)
	if usecolor {
		output = colorable.NewColorableStderr()
	}
	Glogger = log.NewGlogHandler(log.StreamHandler(output, log.TerminalFormat(usecolor)))
}

// Setup initializes profiling and logging based on the CLI flags.
// It should be called as early as possible in the program.
func Setup(ctx *cli.Context) error {
	// logging
	log.PrintOrigins(ctx.GlobalBool(debugFlag.Name))
	Glogger.Verbosity(log.Lvl(ctx.GlobalInt(VerbosityFlag.Name)))
	Glogger.Vmodule(ctx.GlobalString(vmoduleFlag.Name))
	Glogger.BacktraceAt(ctx.GlobalString(backtraceAtFlag.Name))
	log.Root().SetHandler(Glogger)

	// profiling, tracing
	runtime.MemProfileRate = ctx.GlobalInt(memprofilerateFlag.Name)
	Handler.SetBlockProfileRate(ctx.GlobalInt(blockprofilerateFlag.Name))
	if traceFile := ctx.GlobalString(traceFlag.Name); traceFile != "" {
		if err := Handler.StartGoTrace(traceFile); err != nil {
			return err
		}
	}
	if cpuFile := ctx.GlobalString(cpuprofileFlag.Name); cpuFile != "" {
		if err := Handler.StartCPUProfile(cpuFile); err != nil {
			return err
		}
	}
	Handler.filePath = ctx.GlobalString(debugDataDirFlag.Name)

	if periodicProfiling := ctx.GlobalBool(periodicProfilingFlag.Name); periodicProfiling {
		Handler.PeriodicComputeProfiling()
	}

	// pprof server
	if ctx.GlobalBool(pprofFlag.Name) {
		// Hook go-metrics into expvar on any /debug/metrics request, load all vars
		// from the registry into expvar, and execute regular expvar handler.
		exp.Exp(metrics.DefaultRegistry)

		address := fmt.Sprintf("%s:%d", ctx.GlobalString(pprofAddrFlag.Name), ctx.GlobalInt(pprofPortFlag.Name))
		go func() {
			log.Info("Starting pprof server", "addr", fmt.Sprintf("http://%s/debug/pprof", address))
			if err := http.ListenAndServe(address, nil); err != nil {
				log.Error("Failure in running pprof server", "err", err)
			}
		}()
	}
	return nil
}

// Exit stops all running profiles, flushing their output to the
// respective file.
func Exit() {
	Handler.StopCPUProfile()
	Handler.StopGoTrace()
}
