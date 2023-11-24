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

## overview
<img width="757" alt="image" src="https://github.com/josudoey/envoy-playground/assets/15968354/8345d4d7-dd3e-4980-bdbb-1fe5a7fa05f8">
<img width="787" alt="image" src="https://github.com/josudoey/envoy-playground/assets/15968354/b19d6912-7a9a-44eb-9660-ec9bdadff5a1">
