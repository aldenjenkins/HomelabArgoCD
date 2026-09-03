FROM python:3.14.7-alpine3.24 AS base

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY app.py .

# Alpine uses adduser/addgroup (busybox), not useradd
RUN addgroup -g 10001 appuser \
    && adduser -D -H -u 10001 -G appuser appuser

USER 10001
EXPOSE 8080
ENV LOG_LEVEL=INFO

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD python -c "import urllib.request,sys; sys.exit(0 if urllib.request.urlopen('http://127.0.0.1:8080/healthz', timeout=2).status==200 else 1)"

CMD ["gunicorn", "--bind", "0.0.0.0:8080", "--workers", "2", "--access-logfile", "-", "app:app"]
