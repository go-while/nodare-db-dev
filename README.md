# noDare-DB

## Real World Benchmark
### @ Intel Nuc i7-10710U (4.2 Ghz boost)

```
# set/get with verify: Benchmark @ v0.0.2

./client -random=true
{
 parallel: 8
 total: 1000000
 checked: 1000000
 items/round: 125000
 rounds: 8
 insert: 11 sec (90909/sec)
 check: 10 sec (100000/sec)
 total: 21 sec
}

./client -random=false
{
 parallel: 8
 total: 1000000
 checked: 1000000
 items/round: 125000
 rounds: 8
 insert: 10 sec (100000/sec)
 check: 11 sec (90909/sec)
 total: 21 sec
}

with my cpu clock limited to 2.00 Ghz =)
```

### Mode 1 HTTP
```
./client -mode=1
2024/06/16 23:30:15 [INFO] insert finished: took 25 sec! checking...
2024/06/16 23:30:30 [INFO] Check done! Test Result:
{
 parallel: 8
 total: 1000000
 checked: 1000000
 items/round: 125000
 rounds: 8
 insert: 25 sec (40000/sec)
 check: 15 sec (66666/sec)
 total: 40 sec
}
```


### Mode 2 Raw TCP
```
./client -mode=2
2024/06/15 23:40:38 [INFO] insert finished: took 6 sec! checking...
2024/06/15 23:40:43 [INFO] Check done! Test Result:
{
 parallel: 8
 total: 1000000
 checked: 1000000
 items/round: 125000
 rounds: 8
 insert: 6 sec (166666/sec)
 check: 5 sec (200000/sec)
 total: 11 sec
}
```


**noDare-DB** is a fork of dare-db: a project that provides an in-memory database utilizing Redis-inspired hashtables implemented in Go [here](https://github.com/dmarro89/go-redis-hashtable). It offers a lightweight and efficient solution for storing data in memory and accessing it through simple HTTP operations.

## Project Purpose

The primary goal of this project is to offer an in-memory database that leverages hashtables for efficient data storage and retrieval. The Go implementation allows using this database as a component in other Go services or integrating it into applications that require rapid access to in-memory data.

## Running the Database

### Using Docker

To run the database as a Docker image, ensure you have Docker installed on your system. First, navigate to the root directory of your project and execute the following command to build the Docker image:

```bash
docker build -t nodare-db .
```
Once the image is built, you can run the database as a Docker container with the following command:

```bash
docker run -d -p 2420:2420 -p 2240:2240 nodare-db
```

This command will start the database as a Docker container in detached mode exposing ...
... UDP port 2240 of the container to port ```2240``` on your ```localhost```

... TCP port 2420 of the container to port ```2420``` on your ```localhost```

... TCP port 3420 of the container to port ```3420``` on your ```localhost```

... TCP port 4420 of the container to port ```4420``` on your ```localhost```

### Using TLS Version in Docker

Build special Docker image, which will generate certificates

```bash
docker build --target nodare-db-tls -f Dockerfile.tls.yml .
```

Once the image is built, you can run the database as a Docker container with the following command:

```bash
docker run -d -e NDB_HOST="0.0.0.0" -p "127.0.0.1:2240:2240" -p "127.0.0.1:2420:2420" -p "127.0.0.1:3420:3420" -p "127.0.0.1:4420:4420" -e NDB_PORT=2420 -e NDB_UDP_PORT=2420 -e NDB_SUB_DICKS=1000 -e NDB_TLS_ENABLED="True" -e NDB_TLS_KEY="/app/settings/privkey.pem" -e NDB_TLS_CRT="/app/settings/fullchain.pem" nodare-db-tls
```

Access GET/SET/DEL Commands via API over HTTPS on https://localhost:2420


## How to Use

The in-memory database provides three simple HTTP endpoints to interact with stored data:

### GET /get/{key}

This endpoint retrieves an item from the hashtable using a specific key.

Example usage with cURL:

```bash
curl -X GET http://localhost:2420/get/myKey/dbName
```

### SET /set

This endpoint inserts a new item into the hashtable. The request body should contain the key and value of the new item.

Example usage with cURL:

```bash
curl -X POST -d '{"myKey":"myValue"}' http://localhost:2420/set/dbName
```

### DELETE /del/{key}

This endpoint deletes an item from the hashtable using a specific key.

Example usage with cURL:

```bash
curl -X GET http://localhost:2420/del/myKey/dbName
```


## Example Usage

Below is a simple example of how to use this database in a Go application:

Parameter "/dbName" is optional. Defaults to DB string "0" if not supplied.

```go
package main

import (
    "fmt"
    "net/http"
    "bytes"
)

func main() {
    // Example of inserting a new item
    _, err := http.Post("http://localhost:2420/set/dbName", "application/json", bytes.NewBuffer([]byte(`{"myKey":"myValue"}`)))
    if err != nil {
        fmt.Println("Error while inserting item:", err)
        return
    }

    // Example of retrieving an item
    resp, err := http.Get("http://localhost:2420/get/myKey/dbName")
    if err != nil {
        fmt.Println("Error while retrieving item:", err)
        return
    }
    defer resp.Body.Close()

    // Example of deleting an item
    req, err := http.Get("http://localhost:2420/del/myKey/dbName", nil)
    if err != nil {
        fmt.Println("Error while deleting item:", err)
        return
    }
    _, err = http.DefaultClient.Do(req)
    if err != nil {
        fmt.Println("Error while deleting item:", err)
        return
    }
}


