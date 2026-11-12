from __future__ import annotations

import logging
from concurrent import futures

import grpc
from swarmops.drone.v1 import drone_pb2
from swarmops.planner.v1 import planner_pb2, planner_pb2_grpc

from .survey import DroneStart, plan_survey

log = logging.getLogger(__name__)

DEFAULT_SPACING_M = 20.0
DEFAULT_ALT_M = 40.0


class PlannerService(planner_pb2_grpc.PlannerServiceServicer):
    def PlanSurvey(self, request: planner_pb2.PlanSurveyRequest, context: grpc.ServicerContext):
        alt = request.altitude_m or DEFAULT_ALT_M
        try:
            routes = plan_survey(
                polygon=[(p.lat, p.lon) for p in request.polygon],
                drones=[DroneStart(d.drone_id, d.position.lat, d.position.lon) for d in request.drones],
                spacing_m=request.line_spacing_m or DEFAULT_SPACING_M,
            )
        except ValueError as e:
            context.abort(grpc.StatusCode.INVALID_ARGUMENT, str(e))
        log.info("planned survey: %d drones, %d routes", len(request.drones), len(routes))
        return planner_pb2.PlanSurveyResponse(
            routes=[
                planner_pb2.DroneRoute(
                    drone_id=r.drone_id,
                    waypoints=[drone_pb2.Position(lat=lat, lon=lon, alt_m=alt) for lat, lon in r.waypoints],
                    length_m=r.length_m,
                )
                for r in routes
            ]
        )


def serve(port: int) -> None:
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=8))
    planner_pb2_grpc.add_PlannerServiceServicer_to_server(PlannerService(), server)
    server.add_insecure_port(f"[::]:{port}")
    server.start()
    log.info("planner listening on :%d", port)
    server.wait_for_termination()
