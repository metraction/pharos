# Pharos

## Api

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
- Get all Images: http://localhost:8080/api/docs#/operations/GetAllImages (Without vulnerabilities)
- Get Image with all Details (Vulnerabilities, Packages and Findings from the datase): http://localhost:8080/api/docs#/operations/Getimage (Take any ImageId from the previous step)

```bash
curl --request GET \
  --url http://localhost:8080/api/pharosimagemeta/sha256:1e5f3c5b981a9f91ca91cf13ce87c2eedfc7a083f4f279552084dd08fc477512 \
  --header 'Accept: application/json'
```

> ImageId is not the digest, but some internal id we get from the scanner.
