"""Multi-drone lawnmower (boustrophedon) coverage planning.

1. Project the polygon to a local metric frame (flat-earth around its centroid).
2. Rotate so sweep lines run parallel to the polygon's longest side, which means fewer turns.
3. Clip parallel lines (spaced by the camera footprint) against the polygon.
4. Split the strips into contiguous groups of roughly equal length, one per drone.
5. Assign groups to drones with the Hungarian algorithm (min total transit distance).
"""

from __future__ import annotations

import math
from dataclasses import dataclass

import numpy as np
from scipy.optimize import linear_sum_assignment
from shapely import affinity
from shapely.geometry import LineString, MultiLineString, Polygon

M_PER_DEG = 111_320.0
MAX_LINES = 2_000


@dataclass
class DroneStart:
    drone_id: str
    lat: float
    lon: float


@dataclass
class Route:
    drone_id: str
    waypoints: list[tuple[float, float]]  # (lat, lon)
    length_m: float


class _Frame:
    """Local east/north metres around an origin."""

    def __init__(self, lat0: float, lon0: float) -> None:
        self.lat0, self.lon0 = lat0, lon0
        self.kx = M_PER_DEG * math.cos(math.radians(lat0))

    def to_xy(self, lat: float, lon: float) -> tuple[float, float]:
        return (lon - self.lon0) * self.kx, (lat - self.lat0) * M_PER_DEG

    def to_latlon(self, x: float, y: float) -> tuple[float, float]:
        return self.lat0 + y / M_PER_DEG, self.lon0 + x / self.kx


def _sweep_angle_deg(poly: Polygon) -> float:
    rect = poly.minimum_rotated_rectangle
    xs, ys = rect.exterior.coords.xy
    edges = [(xs[i + 1] - xs[i], ys[i + 1] - ys[i]) for i in range(4)]
    dx, dy = max(edges, key=lambda e: math.hypot(*e))
    return math.degrees(math.atan2(dy, dx))


def _strips(poly: Polygon, spacing: float) -> list[tuple[tuple[float, float], tuple[float, float]]]:
    minx, miny, maxx, maxy = poly.bounds
    n = int((maxy - miny) / spacing) + 1
    if n > MAX_LINES:
        raise ValueError(f"line spacing too small: would generate {n} sweep lines (max {MAX_LINES})")
    strips = []
    for k in range(n):
        y = miny + spacing / 2 + k * spacing
        if y > maxy:
            break
        cut = poly.intersection(LineString([(minx - 1, y), (maxx + 1, y)]))
        parts = cut.geoms if isinstance(cut, MultiLineString) else [cut]
        for p in parts:
            if isinstance(p, LineString) and p.length > 0:
                (x1, _), (x2, _) = p.coords[0], p.coords[-1]
                strips.append(((min(x1, x2), y), (max(x1, x2), y)))
    return strips


def _split(strips: list, n: int) -> list[list]:
    """Contiguous groups with roughly equal total strip length."""
    n = min(n, len(strips))
    lengths = [b[0] - a[0] for a, b in strips]
    share = sum(lengths) / n
    groups: list[list] = [[] for _ in range(n)]
    acc = 0.0
    for s, length in zip(strips, lengths):
        # A strip belongs to whichever share its midpoint falls in.
        k = min(int((acc + length / 2) / share), n - 1)
        groups[k].append(s)
        acc += length
    return [g for g in groups if g]


def _boustrophedon(group: list) -> list[tuple[float, float]]:
    path = []
    for i, (a, b) in enumerate(group):
        path.extend([a, b] if i % 2 == 0 else [b, a])
    return path


def _path_length(pts: list[tuple[float, float]]) -> float:
    return sum(math.dist(pts[i], pts[i + 1]) for i in range(len(pts) - 1))


def plan_survey(polygon: list[tuple[float, float]], drones: list[DroneStart], spacing_m: float) -> list[Route]:
    if len(polygon) < 3:
        raise ValueError("polygon needs at least 3 vertices")
    if not drones:
        raise ValueError("at least one drone is required")
    if spacing_m <= 0:
        raise ValueError("line spacing must be positive")

    lat0 = sum(p[0] for p in polygon) / len(polygon)
    lon0 = sum(p[1] for p in polygon) / len(polygon)
    frame = _Frame(lat0, lon0)

    poly = Polygon([frame.to_xy(lat, lon) for lat, lon in polygon])
    if not poly.is_valid:
        poly = poly.buffer(0)
    if poly.is_empty or poly.area == 0:
        raise ValueError("polygon has no area")

    angle = _sweep_angle_deg(poly)
    rotated = affinity.rotate(poly, -angle, origin=(0, 0))
    groups = _split(_strips(rotated, spacing_m), len(drones))

    def unrotate(pts):
        line = affinity.rotate(LineString(pts) if len(pts) > 1 else LineString(pts * 2), angle, origin=(0, 0))
        return list(line.coords)[: len(pts)]

    paths = [unrotate(_boustrophedon(g)) for g in groups]
    starts = [frame.to_xy(d.lat, d.lon) for d in drones]

    # Each path can be flown forwards or backwards, so the cost is distance to the nearer end.
    cost = np.array([[min(math.dist(s, p[0]), math.dist(s, p[-1])) for p in paths] for s in starts])
    rows, cols = linear_sum_assignment(cost)

    routes = []
    for r, c in zip(rows, cols):
        path = paths[c]
        if math.dist(starts[r], path[-1]) < math.dist(starts[r], path[0]):
            path = path[::-1]
        routes.append(
            Route(
                drone_id=drones[r].drone_id,
                waypoints=[frame.to_latlon(x, y) for x, y in path],
                length_m=_path_length(path),
            )
        )
    return sorted(routes, key=lambda r: r.drone_id)
