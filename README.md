# oauthtoy

## Testing

Notice: Google oauth2 package will automatically request a new access token if the current access token expires in less than 10 seconds.

Server:

    oauthtoy-server

Client:

    oauthtoy-client

Curl:

```
curl -X POST \
  --url 'localhost:8080/oauth/token' \
  --header 'content-type: application/x-www-form-urlencoded' \
  --data grant_type=client_credentials \
  --data client_id=admin \
  --data client_secret=admin \
  --data audience=YOUR_API_IDENTIFIER
```
