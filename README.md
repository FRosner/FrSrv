# FrSrv

## Description

Simple kqueue based TCP echo server.

## Usage

```
# Start the server
go run .
```

```
# Check if we can connect
nc -vz 127.0.0.1 8080

# Send some data to receive echo
curl 127.0.0.1:8080
```

```
# Check open file descriptors
lsof -c FrSrv
```
