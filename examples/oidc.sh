# Script to get the access token and id token from the ADFS OIDC provider. Only for testing purposes.
#!/bin/bash

auth_url="http://127.0.0.1:5556/dex/auth"
token_url="http://127.0.0.1:5556/dex/token"
client_id="example-app"
client_secret="ZXhhbXBsZS1hcHAtc2VjcmV0"
scopes="profile+email+openid+groups"
redirect_port=8000
redirect_uri="http://localhost:$redirect_port"

auth_url=$auth_url?response_type=code\&client_id=$client_id\&redirect_uri=$redirect_uri\&scope=$scopes

echo "Open this url in browser:" $auth_url

echo
echo "Listening on $redirect_uri for the redirect with the authorization code..."
echo "Press Ctrl+C to stop"
nc -l $redirect_port > /tmp/response.txt

auth_code=$(grep "GET /" /tmp/response.txt | awk -F 'code=' '{print $2}' | awk -F ' ' '{print $1}')

if [ -z "${auth_code}" ]; then
  echo "Authorization Code not found"
  exit 1
fi

echo "Authorization Code: ${auth_code}"
echo

response=$(curl -s -X POST "${token_url}" \
  -d "grant_type=authorization_code" \
  -d "code=${auth_code}" \
  -d "redirect_uri=${redirect_uri}" \
  -d "client_id=${client_id}" \
  -d "client_secret=${client_secret}")

access_token=$(echo "${response}" | jq -r '.access_token')
id_token=$(echo "${response}" | jq -r '.id_token')

echo "Access Token: ${access_token}"
echo
echo "ID Token: ${id_token}"