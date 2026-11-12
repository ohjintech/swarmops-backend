import math

import grpc
import pytest
from swarmops.drone.v1 import drone_pb2
from swarmops.planner.v1 import planner_pb2

from planner.server import PlannerService
from planner.survey import DroneStart, plan_survey

# ~400 m x 200 m rectangle near Crissy Field
LAT0, LON0 = 37.8037, -122.4650
DLAT = 200 / 111_320
DLON = 400 / (111_320 * math.cos(math.radians(LAT0)))
RECT = [(LAT0, LON0), (LAT0, LON0 + DLON), (LAT0 + DLAT, LON0 + DLON), (LAT0 + DLAT, LON0)]


def drones(n):
    return [DroneStart(f"d-{i:03d}", LAT0 - 0.001, LON0 + i * 0.0001) for i in range(1, n + 1)]


def test_single_drone_covers_area():
    (route,) = plan_survey(RECT, drones(1), spacing_m=20)
    # 200 m tall / 20 m spacing = 10 sweep lines of ~400 m each
    assert len(route.waypoints) == 20
    assert 4_000 * 0.95 < route.length_m < 4_000 * 1.1 + 200


def test_work_is_split_evenly():
    routes = plan_survey(RECT, drones(4), spacing_m=20)
    assert len(routes) == 4
    lengths = [r.length_m for r in routes]
    assert max(lengths) / min(lengths) < 1.6


def test_more_drones_than_strips():
    routes = plan_survey(RECT, drones(30), spacing_m=50)
    assert 0 < len(routes) <= 4  # 200 m / 50 m = 4 strips


def test_waypoints_stay_near_polygon():
    for r in plan_survey(RECT, drones(3), spacing_m=20):
        for lat, lon in r.waypoints:
            assert LAT0 - 1e-5 <= lat <= LAT0 + DLAT + 1e-5
            assert LON0 - 1e-5 <= lon <= LON0 + DLON + 1e-5


def test_invalid_input():
    with pytest.raises(ValueError):
        plan_survey(RECT[:2], drones(1), spacing_m=20)
    with pytest.raises(ValueError):
        plan_survey(RECT, [], spacing_m=20)
    with pytest.raises(ValueError):
        plan_survey(RECT, drones(1), spacing_m=0.01)


class _Ctx:
    def abort(self, code, details):
        raise grpc.RpcError(code, details)


def test_grpc_handler_builds_response():
    req = planner_pb2.PlanSurveyRequest(
        polygon=[drone_pb2.Position(lat=a, lon=b) for a, b in RECT],
        drones=[planner_pb2.DroneStart(drone_id="d-001", position=drone_pb2.Position(lat=LAT0, lon=LON0))],
        line_spacing_m=25,
        altitude_m=35,
    )
    res = PlannerService().PlanSurvey(req, _Ctx())
    assert len(res.routes) == 1
    assert all(w.alt_m == 35 for w in res.routes[0].waypoints)
