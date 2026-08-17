When upgrading the docker image:

1. run these:

```
docker build -t registry.home.alden.ai/me/nautobot:3.2.3-bgp1 -f Dockerfile .

docker push registry.home.alden.ai/me/nautobot:3.2.3-bgp1

k scale -n nautobot deploy/nautobot deploy/nautobot-celery deploy/nautobot-celery-beat --replicas=0

k delete job -n nautobot nautobot-init
```

2. Comment the server/celery/celery-beat in kustomization.yaml and uncomment the `init` line

3. run kdiff and kapply if the only change is the new init job

4. watch `k logs -n nautobot job/nautobot-init -f`

5. run

```
k delete job -n nautobot nautobot-init
```

6. comment init again and uncomment the server/celery/celery-beat

7. update the tag in server/celery/celery-beat

8. rerun kdiff and kapply

Done@
