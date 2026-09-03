# ---- Build stage: has pip, shell, package manager ----
FROM python:3.14.7-alpine3.24 AS builder
WORKDIR /app

COPY requirements.txt .
# Install into a target dir we can copy cleanly into distroless
RUN pip install --no-cache-dir --target=/app/deps -r requirements.txt

COPY app.py .

# ---- Final stage: no shell, no package manager, minimal attack surface ----
FROM gcr.io/distroless/python3-debian12:nonroot AS base
WORKDIR /app

# Copy installed packages and app code from the builder
COPY --from=builder /app/deps /app/deps
COPY --from=builder /app/app.py .

# Make the installed packages importable
ENV PYTHONPATH=/app/deps
ENV LOG_LEVEL=INFO
ENV PYTHONDONTWRITEBYTECODE=1
# gunicorn 26+ writes a control socket to $HOME/.gunicorn on startup.
# /home/nonroot is read-only under readOnlyRootFilesystem, so point
# HOME at the /tmp emptyDir already mounted in the deployment.
ENV HOME=/tmp

# distroless "nonroot" variant already runs as uid 65532 (user "nonroot"),
# no useradd/adduser step needed
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["-c", "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://127.0.0.1:8080/healthz', timeout=2).status==200 else 1)"]

# No shell in distroless, so exec form is mandatory (this was already exec form, good)
# gunicorn must be resolved as a module since there's no gunicorn binary on PATH
CMD ["-m", "gunicorn", "--bind", "0.0.0.0:8080", "--workers", "2", "--access-logfile", "-", "--no-control-socket", "app:app"]
