File Format Conversion

https://github.com/C4illin/ConvertX

NOTE: the Dockerfile here is for building the image with vaapi hardware acceleration support.

```
docker build -t registry.home.alden.ai/me/convertx:v0.17.0-vaapi -f Dockerfile .
docker push registry.home.alden.ai/me/convertx:v0.17.0-vaapi
```
