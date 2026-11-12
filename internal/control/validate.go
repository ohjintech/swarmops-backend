package control

import (
	"errors"
	"fmt"

	dronev1 "github.com/swarm-ops/swarmops-contracts/gen/go/swarmops/drone/v1"
)

// MaxAltitudeM is the regulatory ceiling (FAA Part 107: 400 ft AGL).
const MaxAltitudeM = 120.0

func validatePosition(p *dronev1.Position) error {
	if p == nil {
		return errors.New("position required")
	}
	if p.GetLat() < -90 || p.GetLat() > 90 || p.GetLon() < -180 || p.GetLon() > 180 {
		return fmt.Errorf("position out of range: %v,%v", p.GetLat(), p.GetLon())
	}
	if p.GetAltM() < 0 || p.GetAltM() > MaxAltitudeM {
		return fmt.Errorf("altitude must be within 0..%.0fm", MaxAltitudeM)
	}
	return nil
}

// validateCommand performs server-side checks that do not depend on drone state.
// Geofence / no-fly-zone checks are added here once geofences are persisted.
func validateCommand(c *dronev1.Command) error {
	switch k := c.GetKind().(type) {
	case *dronev1.Command_Takeoff:
		if alt := k.Takeoff.GetAltM(); alt < 2 || alt > MaxAltitudeM {
			return fmt.Errorf("takeoff altitude must be within 2..%.0fm", MaxAltitudeM)
		}
	case *dronev1.Command_GoTo:
		return validatePosition(k.GoTo.GetTarget())
	case *dronev1.Command_FollowRoute:
		wps := k.FollowRoute.GetWaypoints()
		if len(wps) == 0 {
			return errors.New("route has no waypoints")
		}
		for i, w := range wps {
			if err := validatePosition(w); err != nil {
				return fmt.Errorf("waypoint %d: %w", i, err)
			}
		}
	case *dronev1.Command_Hold, *dronev1.Command_ReturnToLaunch, *dronev1.Command_Land:
	case nil:
		return errors.New("command kind required")
	default:
		return fmt.Errorf("unsupported command %T", k)
	}
	return nil
}
