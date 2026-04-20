# Redlib

Private reddit frontend

https://github.com/redlib-org/redlib

NOTE:

There's a current issue where the latest image fails to work with the current rust ssl library so the Dockerfile here fixes that

```
docker build -t registry.home.alden.ai/me/redlib:v0.36.0 -f Dockerfile .
docker push registry.home.alden.ai/me/redlib:v0.36.0
```
