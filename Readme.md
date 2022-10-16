# envoy-playground

envoy proxy example

## example start

```sh
docker-compose up
```

## example upstream server 
```sh
open http://localhost:8080
```

## example proxy server 
```sh
open http://localhost:10000
# default 
#   username: guest
#   password: guest
```

## example proxy admin 
```sh
open http://localhost:9901
```

## rebuild image 
```sh
docker-compose build
```
