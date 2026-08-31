import json
import logging
import os
import time
import urllib.parse

from flask import Flask, Response, redirect, request

app = Flask(__name__)
logging.basicConfig(level=os.environ.get("LOG_LEVEL", "INFO"))
log = logging.getLogger("mimir-redirect")

# Required
GRAFANA_URL = os.environ["GRAFANA_URL"].rstrip("/")
MIMIR_DS_UID = os.environ["MIMIR_DS_UID"]

# Optional
GRAFANA_ORG_ID = os.environ.get("GRAFANA_ORG_ID", "1")
LOOKBACK_MS = int(os.environ.get("LOOKBACK_MINUTES", "60")) * 60 * 1000


def build_explore_url(expr: str) -> str:
    now_ms = int(time.time() * 1000)
    panes = {
        "a": {
            "datasource": MIMIR_DS_UID,
            "queries": [
                {
                    "refId": "A",
                    "expr": expr,
                    "datasource": {"type": "prometheus", "uid": MIMIR_DS_UID},
                }
            ],
            "range": {"from": str(now_ms - LOOKBACK_MS), "to": str(now_ms)},
        }
    }
    query = urllib.parse.urlencode(
        {
            "schemaVersion": "1",
            "panes": json.dumps(panes, separators=(",", ":")),
            "orgId": GRAFANA_ORG_ID,
        }
    )
    return f"{GRAFANA_URL}/explore?{query}"


@app.get("/healthz")
def healthz() -> Response:
    return Response("ok", status=200)


@app.get("/graph")
@app.get("/alertmanager/graph")
def graph() -> Response:
    expr = request.args.get("g0.expr")
    if not expr:
        return Response(
            "missing g0.expr query parameter", status=400, mimetype="text/plain"
        )
    url = build_explore_url(expr)
    log.info("redirecting expr=%r -> %s", expr, url)
    return redirect(url, code=302)


@app.get("/")
def index() -> Response:
    return Response(
        "mimir-redirect: converts Prometheus classic-UI graph links "
        "(/graph?g0.expr=...) into Grafana Explore links.\n",
        mimetype="text/plain",
    )


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8080)
