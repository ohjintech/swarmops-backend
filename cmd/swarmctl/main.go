// swarmctl is an operator CLI for the control API.
//
//	swarmctl drones
//	swarmctl takeoff d-001 [alt_m]
//	swarmctl goto d-001 <lat> <lon> [alt_m]
//	swarmctl hold|rtl|land d-001
//	swarmctl status <command_id>
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"connectrpc.com/connect"

	controlv1 "github.com/swarm-ops/swarmops-contracts/gen/go/swarmops/control/v1"
	"github.com/swarm-ops/swarmops-contracts/gen/go/swarmops/control/v1/controlv1connect"
	dronev1 "github.com/swarm-ops/swarmops-contracts/gen/go/swarmops/drone/v1"
)

func main() {
	api := flag.String("api", envOr("SWARMOPS_API", "http://localhost:8080"), "control API base URL")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: swarmctl [-api URL] drones | takeoff ID [alt] | goto ID LAT LON [alt] | hold|rtl|land ID | status CMD_ID")
	}
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	client := controlv1connect.NewFleetServiceClient(http.DefaultClient, *api)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := run(ctx, client, args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, c controlv1connect.FleetServiceClient, args []string) error {
	switch args[0] {
	case "drones":
		res, err := c.ListDrones(ctx, connect.NewRequest(&controlv1.ListDronesRequest{}))
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "DRONE\tSTATE\tBATTERY\tALT\tLAT\tLON\tSTALE")
		for _, d := range res.Msg.GetDrones() {
			t := d.GetLatest()
			fmt.Fprintf(w, "%s\t%s\t%.0f%%\t%.1f\t%.6f\t%.6f\t%v\n", t.GetDroneId(), shortState(t.GetState()),
				t.GetBatteryPct(), t.GetPosition().GetAltM(), t.GetPosition().GetLat(), t.GetPosition().GetLon(), d.GetStale())
		}
		return w.Flush()

	case "status":
		if len(args) < 2 {
			return fmt.Errorf("status needs a command id")
		}
		res, err := c.GetCommand(ctx, connect.NewRequest(&controlv1.GetCommandRequest{CommandId: args[1]}))
		if err != nil {
			return err
		}
		printRecord(res.Msg.GetCommand())
		return nil
	}

	if len(args) < 2 {
		return fmt.Errorf("%s needs a drone id", args[0])
	}
	cmd, err := buildCommand(args[0], args[2:])
	if err != nil {
		return err
	}
	res, err := c.SendCommand(ctx, connect.NewRequest(&controlv1.SendCommandRequest{DroneId: args[1], Command: cmd}))
	if err != nil {
		return err
	}
	printRecord(res.Msg.GetCommand())
	return nil
}

func buildCommand(verb string, rest []string) (*dronev1.Command, error) {
	num := func(i int, def float64) (float64, error) {
		if i >= len(rest) {
			return def, nil
		}
		return strconv.ParseFloat(rest[i], 64)
	}
	switch verb {
	case "takeoff":
		alt, err := num(0, 30)
		return &dronev1.Command{Kind: &dronev1.Command_Takeoff{Takeoff: &dronev1.Takeoff{AltM: alt}}}, err
	case "goto":
		if len(rest) < 2 {
			return nil, fmt.Errorf("goto needs LAT LON")
		}
		lat, err1 := num(0, 0)
		lon, err2 := num(1, 0)
		alt, err3 := num(2, 0)
		for _, e := range []error{err1, err2, err3} {
			if e != nil {
				return nil, e
			}
		}
		return &dronev1.Command{Kind: &dronev1.Command_GoTo{GoTo: &dronev1.GoTo{
			Target: &dronev1.Position{Lat: lat, Lon: lon, AltM: alt},
		}}}, nil
	case "hold":
		return &dronev1.Command{Kind: &dronev1.Command_Hold{Hold: &dronev1.Hold{}}}, nil
	case "rtl":
		return &dronev1.Command{Kind: &dronev1.Command_ReturnToLaunch{ReturnToLaunch: &dronev1.ReturnToLaunch{}}}, nil
	case "land":
		return &dronev1.Command{Kind: &dronev1.Command_Land{Land: &dronev1.Land{}}}, nil
	}
	return nil, fmt.Errorf("unknown command %q", verb)
}

func printRecord(r *controlv1.CommandRecord) {
	fmt.Printf("%s  drone=%s  status=%s", r.GetCommandId(), r.GetDroneId(), r.GetStatus())
	if r.GetReason() != "" {
		fmt.Printf("  reason=%q", r.GetReason())
	}
	fmt.Println()
}

func shortState(s dronev1.FlightState) string {
	name := s.String()
	const prefix = "FLIGHT_STATE_"
	if len(name) > len(prefix) {
		return name[len(prefix):]
	}
	return name
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
