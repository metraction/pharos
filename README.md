# Pharos

## Api

- I suggest you remove all tables from your database and then start the api again. (but not necessary, just leaves some unused columns or tables)
- Start two scanners, one for sync and one for async:

`go run main.go scanner --scanner.requestQueue priorityScantasks --scanner.responseQueue priorityScanresult`
`go run main.go scanner --scanner.requestQueue priorityScantasks --scanner.responseQueue priorityScanresult`

- Point your browser to: http://localhost:8080/api/docs
- Submit a scan task with sync scan: http://localhost:8080/api/docs#/operations/SyncScan (you can use the simple example provided)

```bash
curl --request POST \
  --url http://localhost:8080/api/pharosscantask/syncscan \
  --header 'Accept: application/json' \
  --header 'Content-Type: application/json' \
  --data '{
"imageSpec": {
"image": "redis:latest"
}
}'
```

- The Scanner returns the scan result and saves to the database.

- Do an async scan: http://localhost:8080/api/docs#/operations/AsyncScan

```bash
curl --request POST \
  --url http://localhost:8080/api/pharosscantask/asyncsyncscan \
  --header 'Accept: application/json' \
  --header 'Content-Type: application/json' \
  --data '{
"imageSpec": {
"image": "nginx:latest"
}
}'
```

- Get all Images: http://localhost:8080/api/docs#/operations/GetAllImages (Without vulnerabilities)
- Get Image with all Details (Vulnerabilities, Packages and Findings from the datase): http://localhost:8080/api/docs#/operations/Getimage (Take any ImageId from the previous step)

```bash
curl --request GET \
  --url http://localhost:8080/api/pharosimagemeta/sha256:1e5f3c5b981a9f91ca91cf13ce87c2eedfc7a083f4f279552084dd08fc477512 \
  --header 'Accept: application/json'
```

> ImageId is not the digest, but some internal id we get from the scanner. So you have to find it by getting all images. (we will provide a function later.)

You can also use Swagger at http://localhost:8080/api/swagger