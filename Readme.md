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
open http://localhost:10000/example
open http://localhost:10000/basic/example
# default 
#   username: guest
#   password: guest
open http://localhost:10000/bearer/example
```

## example proxy admin 
```sh
open http://localhost:9901
```

## rebuild image 
```sh
docker-compose build
```
